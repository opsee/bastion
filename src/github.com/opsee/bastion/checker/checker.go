package checker

import (
	"fmt"
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
	logger.Debug("unmarshaled Any to: %s", instance)

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
	Scheduler  *Scheduler
	grpcServer *grpc.Server
}

func NewChecker() *Checker {
	return &Checker{
		grpcServer: grpc.NewServer(),
		Scheduler:  NewScheduler(),
	}
}

// TODO: One way or another, all CRUD requests should be transactional.

func (c *Checker) invoke(ctx context.Context, cmd string, req *CheckResourceRequest) (*ResourceResponse, error) {
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

func buildURL(check *HttpCheck, target *string) string {
	return fmt.Sprintf("%s://%s:%d%s", check.Protocol, *target, check.Port, check.Path)
}

func (c *Checker) TestCheck(ctx context.Context, req *TestCheckRequest) (*TestCheckResponse, error) {
	var (
		cancel context.CancelFunc
	)
	logger.Debug("Received request: %s", req)

	if req.Deadline == nil {
		return nil, fmt.Errorf("Deadline required but missing in request. %v", req)
	}

	deadline := time.Unix(req.Deadline.Seconds, req.Deadline.Nanos)
	ctx, cancel = context.WithDeadline(ctx, deadline)
	ctx = context.WithValue(ctx, "MaxHosts", int(req.MaxHosts))

	
	responses, errs := c.Scheduler.RunCheck(ctx, req.Check)
	defer close(errs)

	testCheckResponse := &TestCheckResponse{
		Responses: make([]*Any, req.MaxHosts),
	}

	for i := 0; i < int(req.MaxHosts); i++ {
		logger.Debug("Waiting for response %d", i)
		select {
		case response := <-responses:
			if response != nil {
				logger.Debug("Got response: %s", response)

				responseAny, err := MarshalAny(response)
				if err != nil {
					logger.Error("Error marshalling response: ", err)
					cancel()
					return testCheckResponse, err
				}
				testCheckResponse.Responses[i] = responseAny
			}
		case err := <-errs:
			logger.Error("Got an error: %v", err.Error())
			cancel()
			return testCheckResponse, err
		case <-ctx.Done():
			err := ctx.Err()
			return testCheckResponse, err
		}
	}

	logger.Debug("Results: %v", testCheckResponse)
	return testCheckResponse, nil
}

func (c *Checker) Start() error {
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
