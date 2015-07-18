package checker

import (
	"encoding/json"
	"math"
	"time"

	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
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
	Name     string            `json:"name"`
	Id       string            `json:"id"`
	Interval int               `json:"interval"` // in seconds
	URL      string            `json:"url"`
	Protocol string            `json:"protocol"`
	Verb     string            `json:"verb"`
	Target   Target            `json:"target"`
	Headers  map[string]string `json:"headers"`
	Body     string            `json:"body"`
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
	consumer   *messaging.Consumer
	producer   *messaging.Producer
	dispatcher *Dispatcher
	resolver   *Resolver
}

func NewChecker(config *config.Config) (*Checker, error) {
	consumer, err := messaging.NewConsumer("commands", "checker")
	if err != nil {
		return nil, err
	}

	producer, err := messaging.NewProducer("results")
	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}

	checker := &Checker{
		consumer:   consumer,
		producer:   producer,
		dispatcher: NewDispatcher(),
		resolver:   NewResolver(config),
	}

	return checker, nil
}

func (c *Checker) Create(check Check) error {

	return nil
}

func (c *Checker) Delete(check Check) error {

	return nil
}

func (c *Checker) Update(check Check) error {

	return nil
}

func buildURL(check Check, target Target) string {
	return ""
}

func (c *Checker) Test(check Check) {
	var (
		targets []Target
		err     error
	)

	if targets, err = c.resolver.Resolve(check.Target); err != nil {
		logger.Error("Unable to resolve target: ", check.Target)
		logger.Error(err.Error())
		return
	}

	numTargets := int(math.Min(5, float64(len(targets))))
	targets = targets[0 : numTargets-1]

	responses := make(chan Response, numTargets)
	go func() {
		for _, target := range targets {
			if target.Type == "instance" {
				t := buildURL(check, target)

				request := &HTTPRequest{
					Method:  check.Verb,
					Target:  t,
					Headers: check.Headers,
					Body:    check.Body,
				}
				task := &Task{
					Type:     "HTTPRequest",
					Request:  request,
					Response: responses,
				}
				c.dispatcher.Requests <- task
			} else {
				logger.Error("Target did not resolve to instance: ", target)
			}
		}
	}()

	go func() {
		defer close(responses)
		defer func() {
			if r := recover(); r != nil {
				logger.Error("%s: %s", r, check)
			}
		}()
		for {
			select {
			case response := <-responses:
				result := Result{
					CheckId:  check.Id,
					Response: response,
				}
				c.producer.Publish(result)
			case <-time.After(5 * time.Second):
				panic("test_check timed out after 5 seconds")
			}
		}
	}()
}

func (c *Checker) Start() {
	go func() {
		for event := range c.consumer.Channel() {
			command := new(CheckCommand)
			if err := json.Unmarshal([]byte(event.Body()), command); err != nil {
				logger.Error("Cannot unmarshal command: ", event.Body())
			} else {
				switch command.Action {
				case "test_check":
					c.Test(command.Check)
				case "create_check":
					c.Create(command.Check)
				case "update_check":
					c.Update(command.Check)
				case "delete_check":
					c.Delete(command.Check)
				}
			}
		}
	}()
}

func (c *Checker) Stop() {
	c.consumer.Stop()
	c.producer.Stop()
}
