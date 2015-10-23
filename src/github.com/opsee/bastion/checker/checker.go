package checker

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
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

// Checker must:
//    - Add a check
//    - Delete a check
//    - Test a check
//    - Inform the scheduler that things need to happen

type Checker struct {
	Port       int
	Consumer   messaging.Consumer
	Producer   messaging.Producer
	Scheduler  *Scheduler
	Runner     *Runner
	grpcServer *grpc.Server
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
	// We add the request deadline here, and the Runner will adhere to that
	// deadline.
	ctx, _ = context.WithDeadline(ctx, deadline)
	ctx = context.WithValue(ctx, "MaxHosts", int(req.MaxHosts))

	testCheckResponse := &TestCheckResponse{}

	responses, err := c.Runner.RunCheck(ctx, req.Check)
	if err != nil {
		testCheckResponse.Error = handleError(err)
		return testCheckResponse, nil
	}

	var responseArr []*CheckResponse
	for response := range responses {
		responseArr = append(responseArr, response)
	}
	testCheckResponse.Responses = responseArr

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
	c.Scheduler.Stop()
}
