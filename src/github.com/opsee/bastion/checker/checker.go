package checker

import (
	"fmt"
	"math"
	"net"
	"reflect"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
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
}

func UnmarshalAny(any *Any) (interface{}, error) {
	class := any.TypeUrl
	bytes := any.Value

	instance := reflect.New(registry[class]).Interface()
	err := proto.Unmarshal(bytes, instance.(proto.Message))
	if err != nil {
		return nil, err
	}
	logger.Debug("instance: %v", instance)

	return instance, nil
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

// Scheduler must:
//    - Add a check
//    - Delete a check
//    - Test a check
//    - Emit messages to the dispatcher that work needs to be done

type Checker struct {
	Port       int
	Consumer   messaging.Consumer
	Producer   messaging.Producer
	Resolver   Resolver
	scheduler  *Scheduler
	dispatcher *Dispatcher
	grpcServer *grpc.Server
}

func NewChecker() *Checker {
	return &Checker{
		dispatcher: NewDispatcher(),
		grpcServer: grpc.NewServer(),
		scheduler:  &Scheduler{},
	}
}

// TODO: One way or another, all CRUD requests should be transactional.

func (c *Checker) invoke(cmd string, req *CheckResourceRequest) (*ResourceResponse, error) {
	responses := make([]*CheckResourceResponse, len(req.Checks))
	response := &ResourceResponse{
		Responses: responses,
	}
	for i, check := range req.Checks {
		in := []reflect.Value{reflect.ValueOf(check)}
		out := reflect.ValueOf(c.scheduler).MethodByName(cmd).Call(in)
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
	return response, nil
}

func (c *Checker) CreateCheck(ctx context.Context, req *CheckResourceRequest) (*ResourceResponse, error) {
	return c.invoke("CreateCheck", req)
}

func (c *Checker) RetrieveCheck(_ context.Context, req *CheckResourceRequest) (*ResourceResponse, error) {
	return c.invoke("RetrieveCheck", req)
}

func (c *Checker) UpdateCheck(_ context.Context, req *CheckResourceRequest) (*ResourceResponse, error) {
	return nil, fmt.Errorf("Not Implemented")
}

func (c *Checker) DeleteCheck(_ context.Context, req *CheckResourceRequest) (*ResourceResponse, error) {
	return c.invoke("DeleteCheck", req)
}

func buildURL(check *HttpCheck, target *string) string {
	return fmt.Sprintf("%s://%s:%d%s", check.Protocol, *target, check.Port, check.Path)
}

func (c *Checker) testHttpCheck(check *HttpCheck, targets []*string, responses chan Response, timer <-chan time.Time) {
	numTargets := len(targets)
	// buffer so that we don't block on insertion, and so that we can hard timeout
	// with a timer and select combo.
	targetChan := make(chan *string, numTargets)
	for _, target := range targets {
		targetChan <- target
	}

	for i := 0; i < numTargets; i++ {
		select {
		case target := <-targetChan:
			logger.Debug("Target acquired: %s", *target)
			if target != nil {
				uri := buildURL(check, target)
				logger.Debug("URL: %s", uri)

				request := &HTTPRequest{
					Method:  check.Verb,
					URL:     uri,
					Headers: check.Headers,
					Body:    check.Body,
				}

				logger.Debug("issuing request: %s", *request)

				task := &Task{
					Type:     "HTTPRequest",
					Request:  request,
					Response: responses,
				}
				c.dispatcher.Requests <- task
			} else {
				logger.Error("Got nil target in Checker.Test, targets: ", targets)
			}
		case <-timer:
			logger.Error("Received timeout while attempting to test HTTP check")
			break
		}
	}
}

func (c *Checker) TestCheck(ctx context.Context, req *TestCheckRequest) (*TestCheckResponse, error) {
	deadline := time.Unix(req.Deadline.Seconds, req.Deadline.Nanos)

	if time.Now().After(deadline) {
		return nil, fmt.Errorf("Deadline expired")
	}

	timer := time.After(deadline.Sub(time.Now()))

	logger.Debug("Received request: %v", req)

	msg, err := UnmarshalAny(req.CheckSpec)
	if err != nil {
		return nil, err
	}

	logger.Debug("Testing check: %v", msg)

	targets, err := c.Resolver.Resolve(req.Target)
	if err != nil {
		errStr := fmt.Sprintf("Resolver error: %s, check: %s ", err, msg)
		logger.Error(errStr)
		return nil, err
	}

	numTargets := int(math.Min(float64(req.MaxHosts), float64(len(targets))))
	logger.Debug("numTargets: %d", numTargets)
	targets = targets[0:numTargets]

	logger.Debug("Checker.TestCheck -- Targets: %v", targets)

	testResult := &TestCheckResponse{
		Responses: make([]*Any, numTargets),
	}

	responses := make(chan Response, numTargets)
	defer close(responses)
	defer func() {
		if r := recover(); r != nil {
			logger.Error("TestCheck panic: %s: %s", r, msg)
		}
	}()

	switch check := msg.(type) {
	case *HttpCheck:
		go c.testHttpCheck(check, targets, responses, timer)
	}

	for i := 0; i < numTargets; i++ {
		select {
		case response := <-responses:
			logger.Debug("Got response: %s", response)

			responseAny, err := MarshalAny(response)
			if err != nil {
				logger.Error("Error marshalling response: %v", err)
				return nil, err
			}
			testResult.Responses[i] = responseAny
		case <-timer:
			return nil, fmt.Errorf("Timed out while testing HTTP check: %v", req)
		}
	}

	logger.Debug("Results: %v", testResult)
	return testResult, nil
}

func (c *Checker) Start() error {
	c.dispatcher.Dispatch()

	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", c.Port))
	if err != nil {
		return err
	}

	RegisterCheckerServer(c.grpcServer, c)
	go c.grpcServer.Serve(listen)

	return nil
}

func (c *Checker) Stop() {
	if c.Consumer != nil {
		if err := c.Consumer.Close(); err != nil {
			logger.Error(err.Error())
		}
	}
	if c.Producer != nil {
		if err := c.Producer.Close(); err != nil {
			logger.Error(err.Error())
		}
	}
	c.grpcServer.Stop()
}
