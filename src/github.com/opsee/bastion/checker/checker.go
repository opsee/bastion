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
	dispatcher *Dispatcher
	grpcServer *grpc.Server
}

func NewChecker() *Checker {
	return &Checker{
		dispatcher: NewDispatcher(),
		grpcServer: grpc.NewServer(),
	}
}

// func (c *Checker) CreateCheck(event messaging.EventInterface, check Check) error {
//
// 	return nil
// }
//
// func (c *Checker) Delete(event messaging.EventInterface, check Check) error {
//
// 	return nil
// }
//
// func (c *Checker) Update(event messaging.EventInterface, check Check) error {
//
// 	return nil
// }

func buildURL(check *HttpCheck, target *string) string {
	return fmt.Sprintf("%s://%s:%d%s", check.Protocol, *target, check.Port, check.Path)
}

func (c *Checker) testHttpCheck(check *HttpCheck, targets []*string, responses chan Response, timer <-chan time.Time) {
	numTargets := len(targets)
	// buffer so that we don't block on insertion.
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

	var (
		numTargets int
	)

	logger.Debug("Received request: %v", req)

	msg, err := UnmarshalAny(req.CheckSpec)
	if err != nil {
		return nil, err
	}

	logger.Debug("Testing check: %v", msg)

	responses := make(chan Response, MaxTestTargets)
	defer close(responses)
	defer func() {
		if r := recover(); r != nil {
			logger.Error("TestCheck panic: %s: %s", r, msg)
		}
	}()

	switch check := msg.(type) {
	case *HttpCheck:
		targets, err := c.Resolver.Resolve(check.Target)
		if err != nil {
			errStr := fmt.Sprintf("Resolver error: %s, check: %s ", err, check)
			logger.Error(errStr)
			return nil, err
		}
		logger.Debug("Checker.TestCheck -- Targets: %v", targets)

		numTargets = int(math.Min(float64(req.MaxHosts), float64(len(targets))))
		logger.Debug("numTargets: %d", numTargets)
		targets = targets[0:numTargets]
		go c.testHttpCheck(check, targets, responses, timer)
	}

	testResult := &TestCheckResponse{
		Responses: make([]*Any, numTargets),
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
