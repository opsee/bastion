package checker

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/coreos/fleet/log"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
)

const (
	MaxTestTargets = 5
)

var (
	logger = logging.GetLogger("checker")
)

type Interval time.Duration

type Target struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Id   string `json:"id"`
}

type Check struct {
	Name     string              `json:"name"`
	Id       string              `json:"id"`
	Interval int                 `json:"interval"` // in seconds
	Path     string              `json:"path"`
	Protocol string              `json:"protocol"`
	Port     int                 `json:"port"`
	Verb     string              `json:"verb"`
	Target   Target              `json:"target"`
	Headers  map[string][]string `json:"headers"`
	Body     string              `json:"body"`
}

type Result struct {
	CheckId  string `json:"check_id"`
	Response Response
}

type CheckCommand struct {
	Action string `json:"action"`
	Check  Check  `json:"parameters"`
}

// Scheduler must:
//    - Add a check
//    - Delete a check
//    - Test a check
//    - Emit messages to the dispatcher that work needs to be done

type Checker struct {
	Consumer   messaging.Consumer
	Producer   messaging.Producer
	Resolver   Resolver
	dispatcher *Dispatcher
}

func NewChecker() *Checker {
	return &Checker{
		dispatcher: NewDispatcher(),
	}
}

func (c *Checker) Create(event messaging.EventInterface, check Check) error {

	return nil
}

func (c *Checker) Delete(event messaging.EventInterface, check Check) error {

	return nil
}

func (c *Checker) Update(event messaging.EventInterface, check Check) error {

	return nil
}

func buildURL(check Check, target string) string {
	return fmt.Sprintf("%s://%s:%d%s", check.Protocol, target, check.Port, check.Path)
}

type TestResult struct {
	CheckId     string     `json:"check_id"`
	ResponseSet []Response `json:"response_set"`
}

func (c *Checker) Test(event messaging.EventInterface, check Check) {
	logger.Debug("Testing check: %s", check)

	targets, err := c.Resolver.Resolve(check.Target)
	if err != nil {
		errStr := fmt.Sprintf("Resolver error: %s, check: %s ", err, check)
		logger.Error(errStr)
		response := &ErrorResponse{
			Error: fmt.Errorf(errStr),
		}
		c.Producer.Publish(response)
		return
	}

	numTargets := int(math.Min(MaxTestTargets, float64(len(targets))))
	logger.Debug("numTargets: %d", numTargets)
	targets = targets[0:numTargets]
	testResult := &TestResult{
		CheckId:     check.Id,
		ResponseSet: make([]Response, numTargets),
	}
	defer event.Reply(testResult)

	responses := make(chan Response, numTargets)
	defer close(responses)
	defer func() {
		if r := recover(); r != nil {
			logger.Error("%s: %s", r, check)
		}
	}()

	go func() {
		for _, target := range targets {
			logger.Debug("Target acquired: %s", *target)
			if target != nil {
				uri := buildURL(check, *target)
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
		}
	}()

	doneChannel := make(chan bool, 1)
	go func() {
		for i := 0; i < numTargets; i++ {
			response := <-responses
			logger.Debug("Got response: %s", response)

			testResult.ResponseSet[i] = response
		}
		doneChannel <- true
	}()

	select {
	case <-doneChannel:
	case <-time.After(30 * time.Second):
		log.Error("Timed out waiting for check test results: %s", check)
	}
}

func (c *Checker) Start() error {
	if c.Consumer == nil {
		consumer, err := messaging.NewConsumer("commands", "checker")
		if err != nil {
			return err
		}
		c.Consumer = consumer
	}

	if c.Producer == nil {
		producer, err := messaging.NewProducer("results")
		if err != nil {
			return err
		}
		c.Producer = producer
	}

	c.dispatcher.Dispatch()

	go func() {
		for event := range c.Consumer.Channel() {
			logger.Debug("Event received: %s", event)
			command := new(CheckCommand)
			if err := json.Unmarshal([]byte(event.Body()), command); err != nil {
				logger.Error("Cannot unmarshal command: %s", event.Body())
			} else {
				switch command.Action {
				case "test_check":
					c.Test(event, command.Check)
				case "create_check":
					c.Create(event, command.Check)
				case "update_check":
					c.Update(event, command.Check)
				case "delete_check":
					c.Delete(event, command.Check)
				}
			}
		}
	}()
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
}
