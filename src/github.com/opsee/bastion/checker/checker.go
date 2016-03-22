package checker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/gogo/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"github.com/nu7hatch/gouuid"
	"github.com/opsee/basic/schema"
	opsee "github.com/opsee/basic/service"
	"github.com/opsee/bastion/auth"
	opsee_types "github.com/opsee/protobuf/opseeproto/types"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	// MaxTestTargets is the maximum number of test-check targets returned
	// from the resolver that we use.
	MaxTestTargets      = 5
	NumCheckSyncRetries = 11

	// BastionProtoVersion is used for feature flagging fields in various Bastion
	// message types that specify a version number.
	BastionProtoVersion = 1
)

var (
	registry = make(map[string]reflect.Type)
)

func init() {
	// Check types for Any recomposition go here.
	registry["HttpCheck"] = reflect.TypeOf(schema.HttpCheck{})

	envLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	if envLevel == "" {
		envLevel = "error"
	}

	logLevel, err := log.ParseLevel(envLevel)
	if err != nil {
		panic(err)
	}
	log.SetLevel(logLevel)
}

// UnmarshalAny unmarshals an Any object based on its TypeUrl type hint.

func UnmarshalAny(any *opsee_types.Any) (interface{}, error) {
	class := any.TypeUrl
	bytes := any.Value

	instance := reflect.New(registry[class]).Interface()
	err := proto.Unmarshal(bytes, instance.(proto.Message))
	if err != nil {
		log.WithFields(log.Fields{"service": "checker", "event": "unmarshall returned error", "error": "couldn't unmarshall *Any"}).Error(err.Error())
		return nil, err
	}
	log.WithFields(log.Fields{"service": "checker", "event": "unmarshal successful"}).Debug("unmarshaled Any to: ", instance)

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
	log.WithFields(log.Fields{"service": "checker", "event": "handleError() error", "error": err}).Warn(string(errStr))

	return string(errStr)
}

// MarshalAny uses reflection to marshal an interface{} into an Any object and
// sets up its TypeUrl type hint.

func MarshalAny(i interface{}) (*opsee_types.Any, error) {
	msg, ok := i.(proto.Message)
	if !ok {
		err := fmt.Errorf("Unable to convert to proto.Message: %v", i)
		log.WithFields(log.Fields{"service": "checker", "event": "marshalling error"}).Error(err.Error())
		return nil, err
	}
	bytes, err := proto.Marshal(msg)

	if err != nil {
		log.WithFields(log.Fields{"service": "checker", "event": "marshalling error"}).Error(err.Error())
		return nil, err
	}

	return &opsee_types.Any{
		TypeUrl: reflect.ValueOf(i).Elem().Type().Name(),
		Value:   bytes,
	}, nil
}

// Interval is the frequency of check execution.

type Interval time.Duration

// RemoteRunner allows you to control a Runner process via NSQ and behaves similarly to the Runner.

type RemoteRunner struct {
	consumer   *nsq.Consumer
	producer   *nsq.Producer
	config     *NSQRunnerConfig
	requestMap map[string]chan *schema.CheckResult // TODO(greg): I really want NBHM for Golang. :(
	sync.RWMutex
}

// NewRemoteRunner configures the Runner and sets up the NSQ message handler.

func NewRemoteRunner(cfg *NSQRunnerConfig) (*RemoteRunner, error) {
	consumer, err := nsq.NewConsumer(cfg.ConsumerQueueName, cfg.ConsumerChannelName, nsq.NewConfig())
	if err != nil {
		log.WithFields(log.Fields{"service": "checker", "event": "NewRemoteRunner", "error": err.Error()}).Error("couldn't create new consumer")
		return nil, err
	}
	producer, err := nsq.NewProducer(cfg.NSQDHost, nsq.NewConfig())
	if err != nil {
		log.WithFields(log.Fields{"service": "checker", "event": "NewRemoteRunner", "error": err.Error()}).Error("couldn't create new producer")
		return nil, err
	}

	r := &RemoteRunner{
		requestMap: make(map[string]chan *schema.CheckResult),
		consumer:   consumer,
		producer:   producer,
		config:     cfg,
	}
	consumer.AddConcurrentHandlers(nsq.HandlerFunc(func(m *nsq.Message) error {
		chk := &schema.CheckResult{}
		err := proto.Unmarshal(m.Body, chk)
		if err != nil {
			log.WithFields(log.Fields{"service": "checker", "event": "NewRemoteRunner", "error": err.Error()}).Error("couldn't add handler function")
			return err
		}

		log.WithFields(log.Fields{"service": "checker", "event": "NewRemoteRunner", "check": chk.String()}).Debug("handling check")

		var respChan chan *schema.CheckResult

		r.withLock(func() {
			respChan = r.requestMap[chk.CheckId]
			log.WithFields(log.Fields{"service": "checker", "event": "NewRemoteRunner", "check": chk.String()}).Debug("got response channel ", respChan)
		})

		if respChan == nil {
			log.WithFields(log.Fields{"service": "checker", "event": "NewRemoteRunner", "check": chk.String()}).Warn("got unexpected results")
			return nil
		}

		// There is a 1:1 mapping of TestCheck calls to CheckResults, so we close
		// the channel here after writing, making it safe to delete the channel
		// once we've returned from RunCheck. We will incur a GC penalty for doing
		// this if the result is never read, but I think we can manage. It might be
		// nice to really understand what the cost of this approach is, but I don't
		// think it's particularly important. -greg
		respChan <- chk
		log.WithFields(log.Fields{"service": "checker", "event": "NewRemoteRunner", "check": chk.String()}).Debug("RemoteRunner handler sent results to channel")
		close(respChan)
		return nil
	}), cfg.MaxHandlers)

	err = consumer.ConnectToNSQD(cfg.NSQDHost)
	if err != nil {
		log.WithFields(log.Fields{"service": "checker", "event": "NewRemoteRunner", "error": err.Error()}).Error("error connecting to nsqd")
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

// RunCheck asynchronously executes the check and blocks waiting on the result. It's important to set a
// context deadline unless you want this to block forever.

func (r *RemoteRunner) RunCheck(ctx context.Context, chk *schema.Check) (*schema.CheckResult, error) {
	log.WithFields(log.Fields{"service": "checker", "event": "RunCheck", "check": chk.String()}).Debug("Running check")

	var (
		id  string
		err error
	)
	if chk.Id == "" {
		uid, err := uuid.NewV4()
		if err != nil {
			log.WithFields(log.Fields{"service": "checker", "event": "RunCheck", "check": chk.String(), "error": err.Error()}).Error("Error creating UUID")
			return nil, err
		}
		id = uid.String()
		chk.Id = id
	} else {
		id = chk.Id
	}

	respChan := make(chan *schema.CheckResult, 1)

	r.withLock(func() {
		r.requestMap[id] = respChan
		log.WithFields(log.Fields{"service": "checker", "event": "RunCheck", "check": chk.String()}).Debug("RemoteRunner.RunCheck: Set response channel for request: ", id)
	})

	defer func() {
		r.withLock(func() {
			delete(r.requestMap, id)
			log.WithFields(log.Fields{"service": "checker", "event": "RunCheck", "check": chk.String()}).Debug("deleted response channel for request: ", id)
		})
	}()

	msg, err := proto.Marshal(chk)
	if err != nil {
		log.WithFields(log.Fields{"service": "checker", "event": "RunCheck", "error": err.Error()}).Error("error marshalling")
		return nil, err
	}

	log.WithFields(log.Fields{"service": "checker", "event": "RunCheck", "check": chk.String()}).Debug("RemoteRunner.RunCheck: publishing request to run check")
	r.producer.Publish(r.config.ProducerQueueName, msg)

	select {
	case result := <-respChan:
		log.WithFields(log.Fields{"service": "checker", "event": "RunCheck", "check": chk.String()}).Debug("RemoteRunner.RunCheck: Got result from resopnse channel: %s", result.String())
		return result, nil
	case <-ctx.Done():
		log.WithFields(log.Fields{"service": "checker", "event": "RunCheck", "error": ctx.Err()}).Error("context error")
		return nil, ctx.Err()
	}
}

// Stop blocks until the NSQ consumer and producer are stopped.

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

// NewChecker sets up the GRPC server for a Checker.

func NewChecker() *Checker {
	return &Checker{
		grpcServer: grpc.NewServer(),
	}
}

func (c *Checker) invoke(ctx context.Context, cmd string, req *opsee.CheckResourceRequest) (*opsee.ResourceResponse, error) {
	log.WithFields(log.Fields{"service": "checker", "event": "invoke", "command": cmd}).Info("handling request")

	responses := make([]*opsee.CheckResourceResponse, len(req.Checks))
	response := &opsee.ResourceResponse{
		Responses: responses,
	}
	for i, check := range req.Checks {
		in := []reflect.Value{reflect.ValueOf(check)}
		out := reflect.ValueOf(c.Scheduler).MethodByName(cmd).Call(in)
		checkResponse, ok := out[0].Interface().(*schema.Check)
		if !ok {
			err, ok := out[1].Interface().(error)
			if ok {
				if err != nil {
					responses[i] = &opsee.CheckResourceResponse{
						Error: err.Error(),
					}
				}
			}
		} else {
			responses[i] = &opsee.CheckResourceResponse{
				Id:    check.Id,
				Check: checkResponse,
			}
		}
	}
	log.WithFields(log.Fields{"service": "checker", "event": "invoke"}).Info("Response: ", response)
	return response, nil
}

// CreateCheck creates a check within a request context. It will return an error if there is any difficulty
// creating the check.

func (c *Checker) CreateCheck(ctx context.Context, req *opsee.CheckResourceRequest) (*opsee.ResourceResponse, error) {
	return c.invoke(ctx, "CreateCheck", req)
}

// RetrieveCheck retrieves an existing check within a request context. It will return an error if the check
// does not exist.

func (c *Checker) RetrieveCheck(ctx context.Context, req *opsee.CheckResourceRequest) (*opsee.ResourceResponse, error) {
	return c.invoke(ctx, "RetrieveCheck", req)
}

// UpdateCheck deletes and then recreates a check within a request context. It will return an error if there is
// a problem deleting or creating a check.

func (c *Checker) UpdateCheck(ctx context.Context, req *opsee.CheckResourceRequest) (*opsee.ResourceResponse, error) {
	c.invoke(ctx, "DeleteCheck", req)
	return c.invoke(ctx, "CreateCheck", req)
}

// DeleteCheck deletes a check within a request context. It will return an error if there is a problem
// deleting the check.

func (c *Checker) DeleteCheck(ctx context.Context, req *opsee.CheckResourceRequest) (*opsee.ResourceResponse, error) {
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

func (c *Checker) TestCheck(ctx context.Context, req *opsee.TestCheckRequest) (*opsee.TestCheckResponse, error) {
	log.WithFields(log.Fields{"service": "checker", "event": "TestCheck"}).Info("Handling request: %v", req)

	if req.Deadline == nil {
		err := fmt.Errorf("Deadline required but missing in request. %v", req)
		log.WithFields(log.Fields{"service": "checker", "event": "TestCheck", "error": err.Error()}).Error("Missing deadline in request!")
		return nil, err
	}

	deadline := time.Unix(req.Deadline.Seconds, int64(req.Deadline.Nanos))
	log.WithFields(log.Fields{"service": "checker", "event": "TestCheck"}).Debug("TestCheck deadline is " + deadline.Sub(time.Now()).String() + " from now.")
	// We add the request deadline here, and the Runner will adhere to that
	// deadline.
	ctx, _ = context.WithDeadline(ctx, deadline)

	testCheckResponse := &opsee.TestCheckResponse{}

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

	log.Debug("Response: %v", testCheckResponse)
	return testCheckResponse, nil
}

// GetExistingChecks will query the backend to retrieve all of the checks for
// this customer's bastion. It authenticates before retrieving the
// configuration.

func (c *Checker) GetExistingChecks(tries int) ([]*schema.Check, error) {
	cache := &auth.BastionAuthCache{Tokens: make(map[string]*auth.BastionAuthToken)}

	var checks = &opsee.CheckResourceRequest{}

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
		log.WithFields(log.Fields{"service": "checker", "Error": err.Error()}).Fatal("Error initializing BastionAuth")
		return nil, err
	} else {
		theauth, header := token.AuthHeader()
		log.WithFields(log.Fields{"service": "checker", "Auth header:": theauth + " " + header}).Info("Synchronizing checks")
		success := false

		for i := 0; i < tries; i++ {
			req, err := http.NewRequest("GET", request.TargetEndpoint, nil)
			if err != nil {
				log.WithFields(log.Fields{"service": "checker", "error": err}).Warn("Couldn't create request to synch checks.")
			} else {
				req.Header.Set("Accept", "application/x-protobuf")
				req.Header.Set(theauth, header)

				timeout := time.Duration(10 * time.Second)
				client := &http.Client{
					Timeout: timeout,
				}
				resp, err := client.Do(req)
				if err != nil {
					log.WithFields(log.Fields{"service": "checker", "error": err, "response": resp}).Error("Couldn't sychronize checks.")
				} else {
					defer resp.Body.Close()
					body, _ := ioutil.ReadAll(resp.Body)
					if err := proto.Unmarshal(body, checks); err != nil {
						log.WithFields(log.Fields{"service": "checker", "error": err, "response": resp}).Error("Couldn't sychronize checks.")
						goto SLEEP
					}

					if resp.StatusCode != http.StatusOK {
						err = errors.New("Non-200 response while synchronizing checks.")
						log.WithFields(log.Fields{"service": "checker", "error": err, "response": resp}).Error("Couldn't sychronize checks.")
						goto SLEEP
					}

					if len(checks.Checks) == 0 {
						// Consider this a fatal error. If there are legitimately 0 checks, then
						// the checker doesn't need to startup anyway. If there are 0 checks because
						// of an error, then same.
						err = errors.New("Got 0 checks when synchronizing bastion state with Bartnet.")
						log.WithFields(log.Fields{"service": "checker", "error": err, "response": resp}).Error("Couldn't sychronize checks.")
						goto SLEEP
					}
					log.Debug("Got existing checks ", checks)
					success = true
					break
				}
			}
		SLEEP:
			time.Sleep((1 << uint(i)) * time.Millisecond * 10)
		}
		if !success {
			return nil, fmt.Errorf("Couldn't synchronize checks.")
		}
	}

	return checks.Checks, nil
}

// Start all of the checker loops, grpc server, etc.
func (c *Checker) Start() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Port))
	if err != nil {
		return err
	}

	// Start the scheduler so that we immediately begin consuming from the run chan
	// otherwise we could fill up that chan and block
	if err := c.Scheduler.Start(); err != nil {
		return err
	}

	log.Info("Getting existing checks")
	// Now Get existing checks from bartnet
	existingChecks, err := c.GetExistingChecks(NumCheckSyncRetries)
	if err != nil {
		log.WithFields(log.Fields{"service": "checker", "event": "sync checks", "error": err}).Error("failed to sync checks")
	}
	for _, check := range existingChecks {
		c.Scheduler.CreateCheck(check)
	}

	// Now start and register the GRPC server and allow users to create/edit/etc checks
	go c.grpcServer.Serve(listen)
	opsee.RegisterCheckerServer(c.grpcServer, c)
	return nil
}

// Stop all of the checker loops, grpc server, etc.
func (c *Checker) Stop() {
	c.Runner.Stop()
	c.grpcServer.Stop()
	c.Scheduler.Stop()
}
