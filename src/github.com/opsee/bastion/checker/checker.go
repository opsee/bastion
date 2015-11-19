package checker

import (
	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/golang/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"github.com/nu7hatch/gouuid"
	"github.com/opsee/bastion/auth"
	"github.com/opsee/bastion/logging"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"reflect"
	"sync"
	"time"
)

const (
	// The
	MaxTestTargets = 5
)

var (
	logger   = logging.GetLogger("checker")
	registry = make(map[string]reflect.Type)
	log      = logrus.New()
)

func init() {
	registry["HttpCheck"] = reflect.TypeOf(HttpCheck{})
	logging.SetLevel("ERROR", "checker")
}

func UnmarshalAny(any *Any) (interface{}, error) {
	class := any.TypeUrl
	bytes := any.Value

	instance := reflect.New(registry[class]).Interface()
	err := proto.Unmarshal(bytes, instance.(proto.Message))
	if err != nil {
		log.WithFields(logrus.Fields{"service": "checker", "event": "unmarshall returned error", "error": "couldn't unmarshall *Any"}).Error(err.Error())
		return nil, err
	}
	log.WithFields(logrus.Fields{"service": "checker", "event": "unmarshal successful"}).Info("unmarshaled Any to: ", instance)

	return instance, nil
}

// This is admittedly janky, but at the very least it gives us some reasonable
// and fast insight into what's going on.
//
// TODO(greg): This should really be handled in a more elegant manner--the number
// of possible error scenarios explodes as the number of components increases.
// So why not strictly define a small set of error types and then categorize all
// errors accordingly? Probably some simple exception classes for the bastion
// that those interacting with the bastion can code around. Until then, just
// fucking slap some shit together.
func handleError(err error) string {
	errMap := map[string]string{}
	switch e := err.(type) {
	default:
		errMap["type"] = "error"
		errMap["error"] = err.Error()
	case awserr.Error:
		errMap["type"] = "aws"
		errMap["code"] = e.Code()
		errMap["error"] = e.Error()
	case awserr.RequestFailure:
		errMap["type"] = "aws"
		errMap["code"] = e.Code()
		errMap["error"] = e.Message()
		errMap["requestId"] = e.RequestID()
	}

	errStr, mErr := json.Marshal(errMap)
	if mErr != nil {
		return `{"type": "error", "error": "cannot determine error"}`
	}
	log.WithFields(logrus.Fields{"service": "checker", "event": "handleError() error", "error": err}).Warn(errStr)

	return string(errStr)
}

func MarshalAny(i interface{}) (*Any, error) {
	msg, ok := i.(proto.Message)
	if !ok {
		err := fmt.Errorf("Unable to convert to proto.Message: %v", i)
		log.WithFields(logrus.Fields{"service": "checker", "event": "marshalling error"}).Error(err.Error())
		return nil, err
	}
	bytes, err := proto.Marshal(msg)

	if err != nil {
		log.WithFields(logrus.Fields{"service": "checker", "event": "marshalling error"}).Error(err.Error())
		return nil, err
	}

	return &Any{
		TypeUrl: reflect.ValueOf(i).Elem().Type().Name(),
		Value:   bytes,
	}, nil
}

type Interval time.Duration

type RemoteRunner struct {
	consumer   *nsq.Consumer
	producer   *nsq.Producer
	config     *NSQRunnerConfig
	requestMap map[string]chan *CheckResult // TODO(greg): I really want NBHM for Golang. :(
	sync.RWMutex
}

func NewRemoteRunner(cfg *NSQRunnerConfig) (*RemoteRunner, error) {
	consumer, err := nsq.NewConsumer(cfg.ConsumerQueueName, cfg.ConsumerChannelName, nsq.NewConfig())
	if err != nil {
		log.WithFields(logrus.Fields{"service": "checker", "event": "NewRemoteRunner", "error": err.Error()}).Error("couldn't create new consumer")
		return nil, err
	}
	producer, err := nsq.NewProducer(cfg.NSQDHost, nsq.NewConfig())
	if err != nil {
		log.WithFields(logrus.Fields{"service": "checker", "event": "NewRemoteRunner", "error": err.Error()}).Error("couldn't create new producer")
		return nil, err
	}

	r := &RemoteRunner{
		requestMap: make(map[string]chan *CheckResult),
		consumer:   consumer,
		producer:   producer,
		config:     cfg,
	}
	consumer.AddConcurrentHandlers(nsq.HandlerFunc(func(m *nsq.Message) error {
		chk := &CheckResult{}
		err := proto.Unmarshal(m.Body, chk)
		if err != nil {
			log.WithFields(logrus.Fields{"service": "checker", "event": "NewRemoteRunner", "error": err.Error()}).Error("couldn't add handler function")
			return err
		}

		log.WithFields(logrus.Fields{"service": "checker", "event": "NewRemoteRunner", "check": chk.String()}).Info("handling check")

		var respChan chan *CheckResult

		r.withLock(func() {
			respChan = r.requestMap[chk.CheckId]
			log.WithFields(logrus.Fields{"service": "checker", "event": "NewRemoteRunner", "check": chk.String()}).Info("got response channel ", respChan)
		})

		if respChan == nil {
			log.WithFields(logrus.Fields{"service": "checker", "event": "NewRemoteRunner", "check": chk.String()}).Warn("got unexpected results")
			return nil
		}

		// There is a 1:1 mapping of TestCheck calls to CheckResults, so we close
		// the channel here after writing, making it safe to delete the channel
		// once we've returned from RunCheck. We will incur a GC penalty for doing
		// this if the result is never read, but I think we can manage. It might be
		// nice to really understand what the cost of this approach is, but I don't
		// think it's particularly important. -greg
		respChan <- chk
		log.WithFields(logrus.Fields{"service": "checker", "event": "NewRemoteRunner", "check": chk.String()}).Info("RemoteRunner handler sent results to channel")
		close(respChan)
		return nil
	}), cfg.MaxHandlers)

	err = consumer.ConnectToNSQD(cfg.NSQDHost)
	if err != nil {
		log.WithFields(logrus.Fields{"service": "checker", "event": "NewRemoteRunner", "error": err.Error()}).Error("error connecting to nsqd")
		return nil, err
	}

	return r, nil
}

func (r *RemoteRunner) withLock(f func()) {
	log.Debug("Acquiring lock on RemoteRunner.")
	r.Lock()
	f()
	r.Unlock()
	log.Debug("Releasing lock on RemoteRunner.")
}

func (r *RemoteRunner) RunCheck(ctx context.Context, chk *Check) (*CheckResult, error) {
	log.WithFields(logrus.Fields{"service": "checker", "event": "RunCheck", "check": chk.String()}).Info("Running check")

	var (
		id  string
		err error
	)
	if chk.Id == "" {
		uid, err := uuid.NewV4()
		if err != nil {
			log.WithFields(logrus.Fields{"service": "checker", "event": "RunCheck", "check": chk.String()}).Error("Check Id empty")
			return nil, err
		}
		id = uid.String()
		chk.Id = id
	} else {
		id = chk.Id
	}

	respChan := make(chan *CheckResult, 1)

	r.withLock(func() {
		r.requestMap[id] = respChan
		log.WithFields(logrus.Fields{"service": "checker", "event": "RunCheck", "check": chk.String()}).Info("RemoteRunner.RunCheck: Set response channel for request: ", id)
	})

	defer func() {
		r.withLock(func() {
			delete(r.requestMap, id)
			log.WithFields(logrus.Fields{"service": "checker", "event": "RunCheck", "check": chk.String()}).Info("deleted response channel for request: ", id)
		})
	}()

	msg, err := proto.Marshal(chk)
	if err != nil {
		log.WithFields(logrus.Fields{"service": "checker", "event": "RunCheck", "error": err.Error()}).Error("error marshalling")
		return nil, err
	}

	log.WithFields(logrus.Fields{"service": "checker", "event": "RunCheck", "check": chk.String()}).Info("RemoteRunner.RunCheck: publishing request to run check")
	r.producer.Publish(r.config.ProducerQueueName, msg)

	select {
	case result := <-respChan:
		log.WithFields(logrus.Fields{"service": "checker", "event": "RunCheck", "check": chk.String()}).Info("RemoteRunner.RunCheck: Got result from resopnse channel: %s", result.String())
		return result, nil
	case <-ctx.Done():
		log.WithFields(logrus.Fields{"service": "checker", "event": "RunCheck", "error": ctx.Err()}).Error("context error")
		return nil, ctx.Err()
	}
}

func (r *RemoteRunner) Stop() {
	r.consumer.Stop()
	<-r.consumer.StopChan
	r.producer.Stop()
}

// Checker must:
//    - Add a check
//    - Delete a check
//    - Test a check
//    - Inform the scheduler that things need to happen

type Checker struct {
	Port       int
	Scheduler  *Scheduler
	grpcServer *grpc.Server
	Runner     *RemoteRunner
}

func NewChecker() *Checker {
	return &Checker{
		grpcServer: grpc.NewServer(),
	}
}

// TODO(greg): One way or another, all CRUD requests should be transactional.
// At the moment it is very much last-write-wins, but consider the following
// scenario:
//
// t(0) -> Checker startup
// t(1) -> Checker initializes configuration sync with opsee
// t(2) -> User updates check id 1 and this gets saved in Bartnet
// t(3) -> Bartnet creates check id 1 on the bastion
// t(4) -> The configuration sync finishes and overwrites check id 1 with
// the previously saved verison.
//
// Now we have dirty writes. So we have to have a way of fixing that. @dan-compton
// suggested timestamps to version check objects. I think that's a good idea. In
// that case, at t(4) above, the old version of check id 1 would be ignored,
// because its timestamp is older than the one sent by bartnet at t(3).

func (c *Checker) invoke(ctx context.Context, cmd string, req *CheckResourceRequest) (*ResourceResponse, error) {
	log.WithFields(logrus.Fields{"service": "checker", "event": "invoke", "command": cmd}).Info("handling request")

	responses := make([]*CheckResourceResponse, len(req.Checks))
	response := &ResourceResponse{
		Responses: responses,
	}
	for i, check := range req.Checks {
		in := []reflect.Value{reflect.ValueOf(check)}
		out := reflect.ValueOf(c.Scheduler).MethodByName(cmd).Call(in)
		checkResponse, ok := out[0].Interface().(*Check)
		if !ok {
			err, ok := out[1].Interface().(error)
			if ok {
				if err != nil {
					responses[i] = &CheckResourceResponse{Error: err.Error()}
				}
			}
		} else {
			responses[i] = &CheckResourceResponse{
				Id:    check.Id,
				Check: checkResponse,
			}
		}
	}
	log.WithFields(logrus.Fields{"service": "checker", "event": "invoke"}).Info("Response: ", response)
	return response, nil
}

func (c *Checker) CreateCheck(ctx context.Context, req *CheckResourceRequest) (*ResourceResponse, error) {
	return c.invoke(ctx, "CreateCheck", req)
}

func (c *Checker) RetrieveCheck(ctx context.Context, req *CheckResourceRequest) (*ResourceResponse, error) {
	return c.invoke(ctx, "RetrieveCheck", req)
}

func (c *Checker) UpdateCheck(_ context.Context, req *CheckResourceRequest) (*ResourceResponse, error) {
	return nil, fmt.Errorf("Not Implemented")
}

func (c *Checker) DeleteCheck(ctx context.Context, req *CheckResourceRequest) (*ResourceResponse, error) {
	return c.invoke(ctx, "DeleteCheck", req)
}

// TestCheck will synchronously execute a check.
//
// A TestCheckResponse is returned if there are no request errors. If there are
// request-specific errors, then an error will be returned with no
// TestCheckResponse.
//
// "Request-specific errors" are defined as:
// - An unresolvable Check target.
// - An unidentifiable Check type or CheckSpec.
//
// TODO(greg): Get this into the invoke() fold so that we can do a "middleware"
// ish pattern. to logging, instrumentation, etc.
func (c *Checker) TestCheck(ctx context.Context, req *TestCheckRequest) (*TestCheckResponse, error) {
	log.WithFields(logrus.Fields{"service": "checker", "event": "TestCheck"}).Info("Handling request: %v", req)

	if req.Deadline == nil {
		err := fmt.Errorf("Deadline required but missing in request. %v", req)
		log.WithFields(logrus.Fields{"service": "checker", "event": "TestCheck", "error": err.Error()}).Info("Missing deadline in request!")
		return nil, err
	}

	deadline := time.Unix(req.Deadline.Seconds, req.Deadline.Nanos)
	log.WithFields(logrus.Fields{"service": "checker", "event": "TestCheck"}).Info("TestCheck deadline is " + deadline.Sub(time.Now()).String() + " from now.")
	// We add the request deadline here, and the Runner will adhere to that
	// deadline.
	ctx, _ = context.WithDeadline(ctx, deadline)

	testCheckResponse := &TestCheckResponse{}

	result, err := c.Runner.RunCheck(ctx, req.Check)
	if err != nil {
		testCheckResponse.Error = handleError(err)
		return testCheckResponse, nil
	}

	responses := result.GetResponses()

	maxHosts := int(req.MaxHosts)
	if maxHosts == 0 {
		maxHosts = len(responses)
	}
	if maxHosts > len(responses) {
		maxHosts = len(responses)
	}

	testCheckResponse.Responses = responses[:maxHosts]

	log.Info("Response: %v", testCheckResponse)
	return testCheckResponse, nil
}

func (c *Checker) GetExistingChecks() (*CheckResourceRequest, error) {
	cache := &auth.BastionAuthCache{Tokens: make(map[string]*auth.BastionAuthToken)}

	var checks = &CheckResourceRequest{}

	tokenType, err := auth.GetTokenTypeByString(os.Getenv("BASTION_AUTH_TYPE"))
	if err != nil {
		return nil, err
	}

	request := &auth.BastionAuthTokenRequest{
		TokenType:        tokenType,
		CustomerEmail:    os.Getenv("CUSTOMER_EMAIL"),
		CustomerPassword: os.Getenv("CUSTOMER_PASSWORD"),
		CustomerID:       os.Getenv("CUSTOMER_ID"),
		TargetEndpoint:   os.Getenv("BARTNET_HOST") + "/checks",
		AuthEndpoint:     os.Getenv("BASTION_AUTH_ENDPOINT"),
	}

	if token, err := cache.GetToken(request); err != nil || token == nil {
		logrus.WithFields(logrus.Fields{"service": "checker", "Error": err.Error()}).Fatal("Error initializing BastionAuth")
		return nil, err
	} else {
		theauth, header := token.AuthHeader()
		logrus.WithFields(logrus.Fields{"service": "checker", "Auth header:": theauth + " " + header}).Info("Synchronizing checks")

		req, err := http.NewRequest("GET", request.TargetEndpoint, nil)
		req.Header.Set("Accept", "application/x-protobuf")
		req.Header.Set(theauth, header)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			logrus.WithFields(logrus.Fields{"service": "checker", "error": err, "response": resp}).Warn("Couldn't sychronize checks")
			return nil, err

		} else {
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			proto.Unmarshal(body, checks)
		}
	}

	return checks, nil
}

func (c *Checker) Start() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Port))
	if err != nil {
		return err
	}

	RegisterCheckerServer(c.grpcServer, c)
	go c.grpcServer.Serve(listen)
	if err := c.Scheduler.Start(); err != nil {
		return err
	}

	return nil
}

func (c *Checker) Stop() {
	c.Runner.Stop()
	c.grpcServer.Stop()
	c.Scheduler.Stop()
}
