package checker

import (
	"fmt"
	"os"
	"reflect"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"golang.org/x/net/context"
)

type NSQRunnerConfig struct {
	ConsumerQueueName   string
	ProducerQueueName   string
	ConsumerChannelName string
	NSQDHost            string
	CustomerID          string
	MaxHandlers         int
}

type NSQRunner struct {
	runner   *Runner
	config   *NSQRunnerConfig
	producer *nsq.Producer
	consumer *nsq.Consumer
}

func NewNSQRunner(runner *Runner, cfg *NSQRunnerConfig) (*NSQRunner, error) {
	consumer, err := nsq.NewConsumer(cfg.ConsumerQueueName, cfg.ConsumerChannelName, nsq.NewConfig())
	if err != nil {
		return nil, err
	}
	producer, err := nsq.NewProducer(cfg.NSQDHost, nsq.NewConfig())
	if err != nil {
		return nil, err
	}

	consumer.AddConcurrentHandlers(nsq.HandlerFunc(func(m *nsq.Message) error {
		// Message is a Check
		// We emit a CheckResult
		check := &Check{}
		if err := proto.Unmarshal(m.Body, check); err != nil {
			return err
		}
		log.WithFields(log.Fields{"check": check.String()}).Debug("Entering NSQRunner handler.")

		timestamp := &Timestamp{
			Seconds: int64(time.Now().Unix()),
		}

		d, err := time.ParseDuration(fmt.Sprintf("%ds", check.Interval))
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(d*2))
		responseChan, err := runner.RunCheck(ctx, check)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{"check": check}).Error("Error running check.")
			cancel()
			result := &CheckResult{
				CustomerId: cfg.CustomerID,
				CheckId:    check.Id,
				CheckName:  check.Name,
				Target:     check.Target,
				Timestamp:  timestamp,
				Responses: []*CheckResponse{&CheckResponse{
					Target: check.Target,
					Error:  handleError(err),
				}},
				Version: BastionProtoVersion,
			}

			msg, err := proto.Marshal(result)
			if err != nil {
				log.WithError(err).Error("Error marshaling CheckResult")
				return err
			}
			if err := producer.Publish(cfg.ProducerQueueName, msg); err != nil {
				log.WithError(err).Error("Error publishing CheckResult")
				return err
			}
			return nil
		}
		// At this point we have successfully run/are running the check. Indicate
		// as such.
		log.WithFields(log.Fields{"check_id": check.Id}).Debug("Runnng check.")

		// TODO(greg): We're currently ignoring the deadline we _just_ set on this
		// this.
		var responses []*CheckResponse
		passing := true
		for response := range responseChan {
			responses = append(responses, response)
			if !response.Passing {
				passing = false
			}
		}

		result := &CheckResult{
			CustomerId: cfg.CustomerID,
			CheckId:    check.Id,
			Target:     check.Target,
			CheckName:  check.Name,
			Timestamp:  timestamp,
			Responses:  responses,
			Passing:    passing,
			Version:    BastionProtoVersion,
		}

		msg, err := proto.Marshal(result)
		if err != nil {
			log.WithError(err).Error("Error marshaling CheckResult")
			cancel()
			return err
		}

		log.Debug("NSQRunner handler publishing result: %s", result.String())
		if err := producer.Publish(cfg.ProducerQueueName, msg); err != nil {
			log.WithError(err).Error("Error publishing CheckResult")
			cancel()
			return err
		}
		log.WithFields(log.Fields{"check_id": check.Id}).Debug("Published result for check")

		return nil
	}), cfg.MaxHandlers)

	err = consumer.ConnectToNSQD(cfg.NSQDHost)
	if err != nil {
		return nil, err
	}

	return &NSQRunner{
		config:   cfg,
		runner:   runner,
		producer: producer,
		consumer: consumer,
	}, nil
}

func (r *NSQRunner) Stop() {
	r.consumer.Stop()
	<-r.consumer.StopChan
	r.producer.Stop()
}

// A Runner is responsible for running checks. Given a request for a check
// (see: RunCheck), it will execute that check within a context, returning
// a response for every resolved check target. The Runner is not meant for
// concurrent use. It provides an asynchronous API for submitting jobs and
// manages its own concurrency.
type Runner struct {
	resolver    Resolver
	dispatcher  *Dispatcher
	slateClient *SlateClient
}

// NewRunner returns a runner associated with a particular resolver.
func NewRunner(resolver Resolver) *Runner {
	dispatcher := NewDispatcher()

	r := &Runner{
		dispatcher: dispatcher,
		resolver:   resolver,
	}

	slateUrl := fmt.Sprintf("http://%s/check", os.Getenv("SLATE_HOST"))
	if slateUrl != "" {
		r.slateClient = NewSlateClient(slateUrl)
	}

	return r
}

func (r *Runner) resolveRequestTargets(ctx context.Context, check *Check) ([]*Target, error) {
	var (
		targets []*Target
		err     error
	)

	if check.Target == nil {
		return nil, fmt.Errorf("resolveRequestTargets: Check requires target. CHECK=%s", check)
	}

	targets, err = r.resolver.Resolve(check.Target)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("No valid targets resolved from %s", check.Target)
	}

	var (
		maxHosts int
		ok       bool
	)
	maxHosts, ok = ctx.Value("MaxHosts").(int)
	if !ok {
		maxHosts = len(targets)
	}
	if maxHosts > len(targets) {
		maxHosts = len(targets)
	}

	return targets[:maxHosts], nil
}

func (r *Runner) dispatch(ctx context.Context, check *Check, targets []*Target) (chan *Task, error) {
	// If the Check submitted is invalid, RunCheck will return a single
	// CheckResponse indicating that there was an error with the Check.
	c, err := UnmarshalAny(check.CheckSpec)
	if err != nil {
		log.WithError(err).Error("dispatch - unable to unmarshal check")
		return nil, err
	}
	log.WithFields(log.Fields{"check": check}).Debug("dispatch check")

	tg := TaskGroup{}

	for _, target := range targets {
		log.WithFields(log.Fields{"target": target}).Debug("dispatch - Handling target.")

		var request Request
		switch typedCheck := c.(type) {
		case *HttpCheck:
			log.WithFields(log.Fields{"target": target}).Debug("dispatch - dispatching for target")
			if target.Address == "" {
				log.WithFields(log.Fields{"target": target}).Error("Target missing address.")
				continue
			}
			uri := fmt.Sprintf("%s://%s:%d%s", typedCheck.Protocol, target.Address, typedCheck.Port, typedCheck.Path)
			request = &HTTPRequest{
				Method:  typedCheck.Verb,
				URL:     uri,
				Headers: typedCheck.Headers,
				Body:    typedCheck.Body,
			}
		default:
			log.WithFields(log.Fields{"type": reflect.TypeOf(c)}).Error("dispatch - Unknown check type.")
			return nil, fmt.Errorf("Unrecognized check type.")
		}

		t := reflect.TypeOf(request).Elem().Name()
		log.WithFields(log.Fields{"request": request, "type": t}).Debug("dispatch - Creating task from request.")

		task := &Task{
			Target:  target,
			Type:    t,
			Request: request,
		}

		log.Debug("dispatch - Dispatching task: %s", *task)

		tg = append(tg, task)
	}

	return r.dispatcher.Dispatch(ctx, tg), nil
}

func (r *Runner) runCheck(ctx context.Context, check *Check, tasks chan *Task, responses chan *CheckResponse) {
	for t := range tasks {
		log.WithFields(log.Fields{"task": *t}).Debug("runCheck - Handling finished task.")
		var (
			responseError string
			responseAny   *Any
			err           error
			passing       bool
		)
		passing = false

		if t.Response.Response != nil {
			responseAny, err = MarshalAny(t.Response.Response)
			if err != nil {
				responseError = err.Error()
			}
		}
		// Overwrite the error if there is an error on the response.
		if t.Response.Error != nil {
			responseError = t.Response.Error.Error()
		}

		response := &CheckResponse{
			Target:   t.Target,
			Response: responseAny,
			Error:    responseError,
		}

		if response.Error == "" && len(check.Assertions) > 0 && r.slateClient != nil {
			passing, err = r.slateClient.CheckAssertions(ctx, check, t.Response.Response)
			if err != nil {
				log.WithError(err).Error("Could not contact slate.")
			}
		}
		log.WithFields(log.Fields{"response": *response}).Debug("runCheck - Emitting CheckResponse.")

		response.Passing = passing
		responses <- response
	}
}

// RunCheck will resolve all of the targets in a check and trigger execution
// against each of the targets. A channel is returned over which the caller
// may receive all of the CheckResponse objects -- 1:1 with the number of
// resolved targets.
//
// RunCheck will return errors immediately if it cannot resolve the check's
// target or if it cannot unmarshal the check body.
//
// If the Context passed to RunCheck includes a MaxHosts value, at most MaxHosts
// CheckResponse objects will be returned.
//
// If the Context passed to RunCheck is cancelled or its deadline is exceeded,
// all CheckResponse objects after that event will be passed to the channel
// with appropriate errors associated with them.
func (r *Runner) RunCheck(ctx context.Context, check *Check) (chan *CheckResponse, error) {
	targets, err := r.resolveRequestTargets(ctx, check)
	if err != nil {
		return nil, err
	}

	tasks, err := r.dispatch(ctx, check, targets)
	if err != nil {
		return nil, err
	}

	responses := make(chan *CheckResponse, 1)
	// TODO: Place requests on a queue, much like the dispatcher. Working from that
	// queue--thus giving us the ability to limit the number of concurrently
	// executing checks.
	go func() {
		defer close(responses)
		r.runCheck(ctx, check, tasks, responses)
	}()
	return responses, nil
}
