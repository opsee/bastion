package checker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/gogo/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"github.com/opsee/basic/schema"
	opsee "github.com/opsee/basic/service"
	"github.com/opsee/bastion/auth"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	opsee_types "github.com/opsee/protobuf/opseeproto/types"
	"github.com/satori/go.uuid"
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
	BastionProtoVersion = 2

	// Time to allow requests to read a response body.
	BodyReadTimeout = 10 * time.Second

	// Maximum length of response bodies
	MaxContentLength = 128000
)

var (
	registry        = make(map[string]reflect.Type)
	metricsRegistry = heart.MetricsRegistry
)

func init() {
	// Check types for Any recomposition go here.
	registry["HttpCheck"] = reflect.TypeOf(schema.HttpCheck{})
	registry["CloudWatchCheck"] = reflect.TypeOf(schema.CloudWatchCheck{})
}

// UnmarshalAny unmarshals an Any object based on its TypeUrl type hint.

func UnmarshalAny(any *opsee_types.Any) (interface{}, error) {
	class := any.TypeUrl
	bytes := any.Value
	t, ok := registry[class]
	if !ok {
		return nil, fmt.Errorf("type not in Any registry: %s", class)
	}

	instance := reflect.New(t).Interface()
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
	log.WithFields(log.Fields{"channel": cfg.ConsumerChannelName, "queue": cfg.ConsumerQueueName}).Debug("Creating RemoteRunner consumer")
	if err != nil {
		log.WithError(err).Error("couldn't create new producer")
		return nil, err
	}
	producer, err := nsq.NewProducer(cfg.ProducerNsqdHost, nsq.NewConfig())
	if err != nil {
		log.WithError(err).Error("couldn't create new producer")
		return nil, err
	}

	log.WithFields(log.Fields{"queue": cfg.ProducerQueueName}).Debug("Creating RemoteRunner producer")

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
			return err
		}

		log.WithFields(log.Fields{"channel": cfg.ConsumerChannelName, "queue": cfg.ConsumerQueueName}).Debugf("Consumed check id %s", chk.CheckId)

		var respChan chan *schema.CheckResult

		r.withLock(func() {
			respChan = r.requestMap[chk.CheckId]
			log.Debugf("found response channel for check id %s", chk.CheckId)
		})

		if respChan == nil {
			log.Debugf("response channel for check id %s is nil", chk.CheckId)
			return nil
		}

		// There is a 1:1 mapping of TestCheck calls to CheckResults, so we close
		// the channel here after writing, making it safe to delete the channel
		// once we've returned from RunCheck. We will incur a GC penalty for doing
		// this if the result is never read, but I think we can manage. It might be
		// nice to really understand what the cost of this approach is, but I don't
		// think it's particularly important. -greg
		respChan <- chk
		close(respChan)
		return nil
	}), cfg.MaxHandlers)

	err = consumer.ConnectToNSQD(cfg.ConsumerNsqdHost)
	if err != nil {
		log.WithError(err).Error("checker's NewRemoteRunner consumer failed to connect to nsqd")
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

func (r *RemoteRunner) RunCheck(ctx context.Context, checkWithTargets *CheckWithTargets) (*schema.CheckResult, error) {
	chk := checkWithTargets.Check
	log.Debugf("RemoteRunner Running check %s", chk.String())

	var (
		id  string
		err error
	)
	if chk.Id == "" {
		id = uuid.NewV4().String()
		chk.Id = id
	} else {
		id = chk.Id
	}

	respChan := make(chan *schema.CheckResult, 1)

	r.withLock(func() {
		r.requestMap[id] = respChan
		log.Debugf("Set response channel for request: %s", id)
	})

	defer func() {
		r.withLock(func() {
			delete(r.requestMap, id)
			log.Debugf("deleted response channel for request: %s", id)
		})
	}()

	msg, err := json.Marshal(checkWithTargets)
	if err != nil {
		log.WithError(err).Error("Failed to marshal checkwithtargets")
		return nil, err
	}

	log.Debug("Publishing request to run check")
	r.producer.Publish(r.config.ProducerQueueName, msg)

	select {
	case result := <-respChan:
		log.Debugf("Got result from response channel: %s", result.String())
		return result, nil
	case <-ctx.Done():
		log.WithError(ctx.Err()).Error("context error")
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
	resolver   Resolver
}

// NewChecker sets up the GRPC server for a Checker.

func NewChecker(r Resolver) *Checker {
	return &Checker{
		grpcServer: grpc.NewServer(),
		resolver:   r,
	}
}

func (c *Checker) invoke(ctx context.Context, cmd string, req *opsee.CheckResourceRequest) (*opsee.ResourceResponse, error) {
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

// TestCheck will synchronously* execute a check.
//
// A TestCheckResponse is returned if there are no request errors. If there are
// request-specific errors, then an error will be returned with no
// TestCheckResponse.
//
// "Request-specific errors" are defined as: - An unresolvable Check target.  -
// An unidentifiable Check type or CheckSpec.
//
// * Synchronously in this case means that TestCheck blocks until a runner has
// returned a response via NSQ or the context deadline is exceeded.
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
	log.WithFields(log.Fields{"service": "checker", "event": "TestCheck"}).Debug("TestCheck deadline is " + deadline.Sub(time.Now().UTC()).String() + " from now.")
	// We add the request deadline here, and the Runner will adhere to that
	// deadline.
	ctx, _ = context.WithDeadline(ctx, deadline)

	testCheckResponse := &opsee.TestCheckResponse{}
	checkWithTargets, err := NewCheckWithTargets(c.resolver, req.Check)
	if err != nil {
		testCheckResponse.Error = handleError(err)
		return testCheckResponse, nil
	}
	result, err := c.Runner.RunCheck(ctx, checkWithTargets)
	// I hate this hot garbage. We have to do this because the Java
	// GRPC client will throw exceptions if we return errors via GRPC.
	// So, rather than dealing with exceptions on the bartnet side, we
	// just do this nonsense.
	// TODO(greg): Fucking get rid of Error fields in GRPC responses.
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

	log.Debugf("Response: %v", testCheckResponse)
	return testCheckResponse, nil
}

// GetExistingChecks will query the backend to retrieve all of the checks for
// this customer's bastion. It authenticates before retrieving the
// configuration.

func (c *Checker) GetExistingChecks(tries int) ([]*schema.Check, error) {
	cache := &auth.BastionAuthCache{Tokens: make(map[string]*auth.BastionAuthToken)}

	var checks = &opsee.CheckResourceRequest{}

	tokenType, err := auth.GetTokenTypeByString(config.GetConfig().BastionAuthType)
	if err != nil {
		return nil, err
	}

	getChecksURL := config.GetConfig().BartnetHost + "/checks"
	if exgid := config.GetConfig().ExecutionGroupId; exgid != "" {
		getChecksURL += "/exgid/" + exgid
	}

	request := &auth.BastionAuthTokenRequest{
		TokenType:      tokenType,
		CustomerEmail:  config.GetConfig().CustomerEmail,
		CustomerID:     config.GetConfig().CustomerId,
		TargetEndpoint: getChecksURL,
		AuthEndpoint:   config.GetConfig().BastionAuthEndpoint,
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
				log.WithFields(log.Fields{"service": "checker", "error": err}).Warn("Couldn't create request to fetch checks.")
			} else {
				req.Header.Set("Accept", "application/x-protobuf")
				req.Header.Set(theauth, header)

				timeout := time.Duration(10 * time.Second)
				client := &http.Client{
					Timeout: timeout,
				}
				resp, err := client.Do(req)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						log.WithError(netErr).Error("Couldn't fetch checks. Dial Timed out.")
					} else if opError, ok := err.(*net.OpError); ok {
						if opError.Op == "dial" {
							log.WithError(netErr).Error("Couldn't sync checks. %s is Unknown Host. Exiting.", request.TargetEndpoint)
							break
						} else if opError.Op == "read" {
							log.WithError(netErr).Errorf("Couldn't fetch checks. Connection to %s Refused.", request.TargetEndpoint)
						}
					} else {
						log.WithError(err).Errorf("Couldn't fetch checks. Failed to connect to %s.", request.TargetEndpoint)
					}
					goto SLEEP
				} else {
					defer resp.Body.Close()
					body, _ := ioutil.ReadAll(resp.Body)
					if err := proto.Unmarshal(body, checks); err != nil {
						log.WithFields(log.Fields{"service": "checker", "error": err, "response": resp}).Error("Couldn't fetch checks.")
						goto SLEEP
					}

					if resp.StatusCode != http.StatusOK {
						err = errors.New("Non-200 response while fetching checks.")
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
			return nil, fmt.Errorf("Couldn't fetch existing checks.")
		}
	}

	return checks.Checks, nil
}

func (c *Checker) synchronizeChecks() error {
	log.Debug("Synchronizing checks with Opsee.")
	// Now Get existing checks from bartnet
	existingChecks, err := c.GetExistingChecks(NumCheckSyncRetries)
	if err != nil {
		return err
	}
	for _, check := range existingChecks {
		c.Scheduler.CreateCheck(check)
	}
	return nil
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

	log.Debug("Getting existing checks")
	err = c.synchronizeChecks()
	if err != nil {
		log.WithError(err).Fatal("Couldn't retrieve existing checks from server.")
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
