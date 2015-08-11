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

	instance := reflect.New(registry[class]).Elem().Interface()
	final := proto.Unmarshal(bytes, instance.(proto.Message))

	return final, nil
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

// func buildURL(check Check, target string) string {
// 	return fmt.Sprintf("%s://%s:%d%s", check.Protocol, target, check.Port, check.Path)
// }

type TestResult struct {
	CheckId     string     `json:"check_id"`
	ResponseSet []Response `json:"response_set"`
}

func (c *Checker) TestCheck(ctx context.Context, req *TestCheckRequest) (*TestCheckResponse, error) {
	logger.Debug("Received request: %v", req)

	check, err := UnmarshalAny(req.CheckSpec)
	if err != nil {
		return nil, err
	}

	logger.Debug("Testing check: %v", check)

	return nil, nil

	// targets, err := c.Resolver.Resolve(check.Target)
	// if err != nil {
	// 	errStr := fmt.Sprintf("Resolver error: %s, check: %s ", err, check)
	// 	logger.Error(errStr)
	// 	response := &ErrorResponse{
	// 		Error: fmt.Errorf(errStr),
	// 	}
	// 	c.Producer.Publish(response)
	// 	return nil, err
	// }
	//
	// numTargets := int(math.Min(MaxTestTargets, float64(len(targets))))
	// logger.Debug("numTargets: %d", numTargets)
	// targets = targets[0:numTargets]
	// testResult := &TestResult{
	// 	CheckId:     check.Id,
	// 	ResponseSet: make([]Response, numTargets),
	// }
	//
	// responses := make(chan Response, numTargets)
	// defer close(responses)
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		logger.Error("%s: %s", r, check)
	// 	}
	// }()
	//
	// go func() {
	// 	for _, target := range targets {
	// 		logger.Debug("Target acquired: %s", *target)
	// 		if target != nil {
	// 			uri := buildURL(check, *target)
	// 			logger.Debug("URL: %s", uri)
	//
	// 			request := &HTTPRequest{
	// 				Method:  check.Verb,
	// 				URL:     uri,
	// 				Headers: check.Headers,
	// 				Body:    check.Body,
	// 			}
	//
	// 			logger.Debug("issuing request: %s", *request)
	//
	// 			task := &Task{
	// 				Type:     "HTTPRequest",
	// 				Request:  request,
	// 				Response: responses,
	// 			}
	// 			c.dispatcher.Requests <- task
	// 		} else {
	// 			logger.Error("Got nil target in Checker.Test, targets: ", targets)
	// 		}
	// 	}
	// }()
	//
	// doneChannel := make(chan bool, 1)
	// go func() {
	// 	for i := 0; i < numTargets; i++ {
	// 		response := <-responses
	// 		logger.Debug("Got response: %s", response)
	//
	// 		testResult.ResponseSet[i] = response
	// 	}
	// 	doneChannel <- true
	// }()
	//
	// select {
	// case <-doneChannel:
	// case <-time.After(30 * time.Second):
	// 	logger.Error("Timed out waiting for check test results: %s", check)
	// }
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
