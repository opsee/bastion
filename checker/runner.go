package checker

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gogo/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"github.com/opsee/basic/schema"
	"github.com/opsee/bastion/config"
	opsee_types "github.com/opsee/protobuf/opseeproto/types"
	metrics "github.com/rcrowley/go-metrics"
	"golang.org/x/net/context"
)

type NSQRunnerConfig struct {
	Id                  string
	ConsumerQueueName   string
	ProducerQueueName   string
	ConsumerChannelName string
	ConsumerNsqdHost    string
	ProducerNsqdHost    string
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
	consumerConfig := nsq.NewConfig()
	// This will effectively be the maximum number of simultaneous Checks that we can
	// run. Keep in mind that each Check MAY yield many requests, and there are only
	// MaxRoutinesPerWorkerType workers per check type.
	consumerConfig.MaxInFlight = 2

	consumer, err := nsq.NewConsumer(cfg.ConsumerQueueName, cfg.ConsumerChannelName, consumerConfig)
	if err != nil {
		return nil, err
	}
	log.Debugf("NSQRunner consuming on queue %s, channel %s", cfg.ConsumerQueueName, cfg.ConsumerChannelName)

	producerConfig := nsq.NewConfig()
	producerConfig.MaxInFlight = 2
	producer, err := nsq.NewProducer(cfg.ProducerNsqdHost, producerConfig)
	if err != nil {
		return nil, err
	}
	log.Debugf("NSQRunner producing on queue %s", cfg.ProducerQueueName)

	registry := metrics.NewPrefixedChildRegistry(metricsRegistry, "runner.")

	consumer.AddConcurrentHandlers(nsq.HandlerFunc(func(m *nsq.Message) error {
		// Message is a Check
		// We emit a CheckResult
		check := &schema.Check{}
		if err := proto.Unmarshal(m.Body, check); err != nil {
			return err
		}
		log.WithFields(log.Fields{"check": check.String()}).Debug("Entering NSQRunner handler.")

		timestamp := &opsee_types.Timestamp{}
		timestamp.Scan(time.Now())

		d, err := time.ParseDuration(fmt.Sprintf("%ds", check.Interval))
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(d*2))
		// A call to RunCheck is synchronous. Calling cancel() is not necessarily superfluous though.
		responses, err := runner.RunCheck(ctx, check)
		log.WithFields(log.Fields{"check_id": check.Id}).Debug("Running check.")
		cancel()
		if err != nil {
			log.WithError(err).WithFields(log.Fields{"check": check}).Error("Error running check.")
			cancel()
			result := &schema.CheckResult{
				CustomerId: cfg.CustomerID,
				CheckId:    check.Id,
				CheckName:  check.Name,
				Target:     check.Target,
				Timestamp:  timestamp,
				Responses: []*schema.CheckResponse{&schema.CheckResponse{
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

			metrics.GetOrRegisterCounter("nsq_messages_handled", registry).Inc(1)

			return nil
		}

		// Determine if the CheckResult has its passing flag set.
		passing := true
		for _, response := range responses {
			if !response.Passing {
				passing = false
			}
		}

		result := &schema.CheckResult{
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
			return err
		}
		if err := producer.Publish(cfg.ProducerQueueName, msg); err != nil {
			log.WithError(err).Error("Error publishing CheckResult")
			return err
		} else {
			log.WithFields(log.Fields{
				"CustomerId": cfg.CustomerID,
				"CheckId":    check.Id,
				"Target":     check.Target,
				"CheckName":  check.Name,
				"Timestamp":  timestamp,
				"Responses":  responses,
				"Passing":    passing,
				"Version":    BastionProtoVersion}).Debug("Published result to queue %s", cfg.ProducerQueueName)
		}

		metrics.GetOrRegisterCounter("nsq_messages_handled", registry).Inc(1)
		return nil
	}), cfg.MaxHandlers)

	err = consumer.ConnectToNSQD(cfg.ConsumerNsqdHost)
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
	registry    metrics.Registry
}

// NewRunner returns a runner associated with a particular resolver.
func NewRunner(resolver Resolver) *Runner {
	dispatcher := NewDispatcher()

	r := &Runner{
		dispatcher: dispatcher,
		resolver:   resolver,
		registry:   metrics.NewPrefixedChildRegistry(metricsRegistry, "runner."),
	}

	slateHost := config.GetConfig().SlateHost
	if slateHost != "" {
		slateUrl := fmt.Sprintf("http://%s/check", slateHost)
		r.slateClient = NewSlateClient(slateUrl)
	}

	return r
}

func (r *Runner) resolveRequestTargets(ctx context.Context, check *schema.Check) ([]*schema.Target, error) {
	var (
		targets []*schema.Target
		err     error
	)

	if check.Target == nil {
		return nil, fmt.Errorf("resolveRequestTargets: Check requires target. CHECK=%s", check)
	}

	targets, err = r.resolver.Resolve(ctx, check.Target)
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

func (r *Runner) dispatch(ctx context.Context, check *schema.Check, targets []*schema.Target) (chan *Task, error) {
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
		case *schema.HttpCheck:
			var (
				host       string
				skipVerify = true
			)

			log.WithFields(log.Fields{"target": target}).Debug("dispatch - dispatching for target")
			if target.Address == "" {
				log.WithFields(log.Fields{"target": target}).Error("Target missing address.")
				continue
			}

			// special case host targets so that we may explicitly set host name in http requests
			// and validate ssl certs
			switch target.Type {
			case "host":
				host = target.Id
				skipVerify = false
			}

			request = &HTTPRequest{
				Method:             typedCheck.Verb,
				URL:                fmt.Sprintf("%s://%s:%d%s", typedCheck.Protocol, target.Address, typedCheck.Port, typedCheck.Path),
				Headers:            typedCheck.Headers,
				Body:               typedCheck.Body,
				Host:               host,
				InsecureSkipVerify: skipVerify,
			}

		case *schema.CloudWatchCheck:
			defaultResponseCacheTTL := time.Second * time.Duration(5)
			cloudwatchCheck, ok := c.(*schema.CloudWatchCheck)
			if !ok {
				return nil, fmt.Errorf("Unable to assert type on cloudwatch check")
			}
			log.WithFields(log.Fields{"target": target}).Debug("dispatch - dispatching for target")

			if target.Id == "" {
				log.WithFields(log.Fields{"target": target}).Error("Target missing Id")
				continue
			}
			if len(cloudwatchCheck.Metrics) == 0 {
				log.Info("Refusing to create CloudWatchCheck with 0 metrics")
				continue
			}

			request = &CloudWatchRequest{
				Target:                 target,
				Metrics:                cloudwatchCheck.Metrics,
				StatisticsIntervalSecs: int(check.Interval * 2),
				StatisticsPeriod:       CloudWatchStatisticsPeriod,
				Statistics:             []string{"Average"}, //TODO(dan) Eventually include all Statistics?
				Namespace:              cloudwatchCheck.Metrics[0].Namespace,
				User:                   r.resolver.GetUser(),
				Region:                 r.resolver.GetRegion(),
				VpcId:                  r.resolver.GetVpc(),
				MaxAge:                 defaultResponseCacheTTL,
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

func (r *Runner) runAssertions(ctx context.Context, check *schema.Check, tasks chan *Task) []*schema.CheckResponse {
	responses := []*schema.CheckResponse{}
	for t := range tasks {
		log.WithFields(log.Fields{"task": *t}).Debug("runAssertions - Handling finished task.")
		var (
			responseError string
			responseAny   *opsee_types.Any
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

		response := &schema.CheckResponse{
			Target:   t.Target,
			Response: responseAny,
			Error:    responseError,
		}

		if response.Error == "" && len(check.Assertions) > 0 && r.slateClient != nil {
			jsonBytes, err := json.Marshal(t.Response.Response)
			passing, err = r.slateClient.CheckAssertions(ctx, check, jsonBytes)
			if err != nil {
				log.WithError(err).Error("Could not contact slate.")
			}
		}
		log.WithFields(log.Fields{"Check Name": check.Name, "Check Id": check.Id}).Debugf("Check is passing: %t", passing)

		response.Passing = passing
		responses = append(responses, response)
	}

	return responses
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
func (r *Runner) RunCheck(ctx context.Context, check *schema.Check) ([]*schema.CheckResponse, error) {
	targets, err := r.resolveRequestTargets(ctx, check)
	if err != nil {
		metrics.GetOrRegisterCounter("resolver_errors", r.registry).Inc(1)
		return nil, err
	}

	// tasks is a channel of tasks which runCheck will iterate over.
	tasks, err := r.dispatch(ctx, check, targets)
	if err != nil {
		return nil, err
	}

	// TODO(greg): Move assertion processing to a parallel model, but for now
	// try to be a little nicer to slate and run these serially, blocking until
	// all assertions have been processed.
	responses := r.runAssertions(ctx, check, tasks)
	return responses, nil
}
