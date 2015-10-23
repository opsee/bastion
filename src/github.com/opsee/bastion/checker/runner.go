package checker

import (
	"fmt"
	"reflect"

	"golang.org/x/net/context"
)

// A Runner is responsible for running checks. Given a request for a check
// (see: RunCheck), it will execute that check within a context, returning
// a response for every resolved check target. The Runner is not meant for
// concurrent use. It provides an asynchronous API for submitting jobs and
// manages its own concurrency.
type Runner struct {
	resolver   Resolver
	dispatcher *Dispatcher
}

// NewRunner returns a runner associated with a particular resolver.
func NewRunner(resolver Resolver) *Runner {
	dispatcher := NewDispatcher()
	return &Runner{
		dispatcher: dispatcher,
		resolver:   resolver,
	}
}

func (r *Runner) resolveRequestTargets(ctx context.Context, check *Check) ([]*Target, error) {
	var (
		targets []*Target
		err     error
	)

	if check.Target.Type != "instance" {
		targets, err = r.resolver.Resolve(check.Target)
		if err != nil {
			return nil, err
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("No valid targets resolved from %s", check.Target)
		}
	} else {
		targets = []*Target{check.Target}
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
		logger.Error("dispatch - unable to unmarshal check: %s", err.Error())
		return nil, err
	}
	logger.Debug("dispatch - check = %s", check)

	tg := TaskGroup{}

	for _, target := range targets {
		logger.Debug("dispatch - Handling target: %s", *target)

		var request Request
		switch typedCheck := c.(type) {
		case *HttpCheck:
			logger.Debug("dispatch - dispatching for target %s", target.Address)
			if target.Address == "" {
				logger.Error("Target missing address: %s", *target)
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
			logger.Error("dispatch - Unknown check type: %s", reflect.TypeOf(c))
			return nil, fmt.Errorf("Unrecognized check type.")
		}

		logger.Debug("dispatch - Creating task from request: %s", request)
		t := reflect.TypeOf(request).Elem().Name()
		logger.Debug("dispatch - Request type: %s", t)

		task := &Task{
			Target:  target,
			Type:    t,
			Request: request,
		}

		logger.Debug("dispatch - Dispatching task: %s", *task)

		tg = append(tg, task)
	}

	return r.dispatcher.Dispatch(ctx, tg), nil
}

func (r *Runner) runCheck(ctx context.Context, check *Check, tasks chan *Task, responses chan *CheckResponse) {
	for t := range tasks {
		logger.Debug("runCheck - Handling finished task: %s", *t)
		var (
			responseError string
			responseAny   *Any
			err           error
		)
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
		logger.Debug("runCheck - Emitting CheckResponse: %s", *response)
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
