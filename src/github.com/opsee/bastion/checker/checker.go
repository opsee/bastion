package checker

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/golang/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"github.com/nu7hatch/gouuid"
	"github.com/opsee/bastion/logging"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	// The
	MaxTestTargets = 5
)

var (
	logger   = logging.GetLogger("checker")
	registry = make(map[string]reflect.Type)
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
		return nil, err
	}
	logger.Debug("unmarshaled Any to: %s", instance)

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

	return string(errStr)
}

func MarshalAny(i interface{}) (*Any, error) {
	msg, ok := i.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("Unable to convert to proto.Message: %v", i)
	}
	bytes, err := proto.Marshal(msg)

	if err != nil {
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
		return nil, err
	}
	producer, err := nsq.NewProducer(cfg.NSQDHost, nsq.NewConfig())
	if err != nil {
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
			logger.Error(err.Error())
			return err
		}

		logger.Debug("RemoteRunner handling check: %s", chk.String())

		var respChan chan *CheckResult

		r.withLock(func() {
			respChan = r.requestMap[chk.CheckId]
			logger.Debug("RemoteRunner handler: Got response channel: %s", respChan)
		})

		if respChan == nil {
			logger.Info("Received unexpected result: %s", chk.String())
			return nil
		}

		// There is a 1:1 mapping of TestCheck calls to CheckResults, so we close
		// the channel here after writing, making it safe to delete the channel
		// once we've returned from RunCheck. We will incur a GC penalty for doing
		// this if the result is never read, but I think we can manage. It might be
		// nice to really understand what the cost of this approach is, but I don't
		// think it's particularly important. -greg
		respChan <- chk
		logger.Debug("RemoteRunner handler sent result to channel.")
		close(respChan)
		return nil
	}), cfg.MaxHandlers)

	err = consumer.ConnectToNSQD(cfg.NSQDHost)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *RemoteRunner) withLock(f func()) {
	logger.Debug("Acquiring lock on RemoteRunner.")
	r.Lock()
	f()
	r.Unlock()
	logger.Debug("Releasing lock on RemoteRunner.")
}

func (r *RemoteRunner) RunCheck(ctx context.Context, chk *Check) (*CheckResult, error) {
	logger.Info("Running check: %s", chk.String())
	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	chk.Id = id.String()

	respChan := make(chan *CheckResult, 1)

	r.withLock(func() {
		r.requestMap[id.String()] = respChan
		logger.Debug("RemoteRunner.RunCheck: Set response channel for request: %s", id.String())
	})

	defer func() {
		r.withLock(func() {
			delete(r.requestMap, id.String())
			logger.Debug("Deleted response channel.")
		})
	}()

	msg, err := proto.Marshal(chk)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	logger.Debug("RemoteRunner.RunCheck: publishing request to run check: %s", chk.String())
	r.producer.Publish(r.config.ProducerQueueName, msg)

	select {
	case result := <-respChan:
		logger.Debug("RemoteRunner.RunCheck: Got result from resopnse channel: %s", result.String())
		return result, nil
	case <-ctx.Done():
		logger.Error(ctx.Err().Error())
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

// TODO: One way or another, all CRUD requests should be transactional.

func (c *Checker) invoke(ctx context.Context, cmd string, req *CheckResourceRequest) (*ResourceResponse, error) {
	logger.Info("Handling request: %s", req)

	responses := make([]*CheckResourceResponse, len(req.Checks))
	response := &ResourceResponse{
		Responses: responses,
	}
	for i, check := range req.Checks {
		in := []reflect.Value{reflect.ValueOf(check)}
		out := reflect.ValueOf(c.Scheduler).MethodByName(cmd).Call(in)
		checkResponse := out[0].Interface().(*Check)
		err := out[1].Interface().(error)
		if err != nil {
			responses[i] = &CheckResourceResponse{Error: err.Error()}
		}
		responses[i] = &CheckResourceResponse{
			Id:    check.Id,
			Check: checkResponse,
		}
	}
	logger.Info("Response: %s", response)
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
	logger.Info("Handling request: %s", req)

	if req.Deadline == nil {
		return nil, fmt.Errorf("Deadline required but missing in request. %v", req)
	}
	deadline := time.Unix(req.Deadline.Seconds, req.Deadline.Nanos)
	logger.Debug("TestCheck deadline is %d from now.", deadline.Sub(time.Now()).String())
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

	logger.Info("Response: %v", testCheckResponse)
	return testCheckResponse, nil
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
