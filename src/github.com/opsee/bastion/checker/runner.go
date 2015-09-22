package checker

import (
	"fmt"
	"reflect"

	"golang.org/x/net/context"
)

// A Runner is responsible for running checks and
type Runner struct {
	resolver   Resolver
	dispatcher *Dispatcher
}

func NewRunner(resolver Resolver) *Runner {
	dispatcher := NewDispatcher()
	return &Runner{
		dispatcher: dispatcher,
		resolver:   resolver,
	}
}

func (r *Runner) resolveRequestTargets(ctx context.Context, check *Check) (chan *Target, error) {
	var (
		targets []*Target
		err     error
	)

	out := make(chan *Target)

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

	go func() {
		defer close(out)

		var (
			maxHosts int
			ok       bool
		)

		maxHosts, ok = ctx.Value("MaxHosts").(int)
		if !ok {
			maxHosts = len(targets)
		}
		logger.Debug("resolveRequestTargets: MaxHosts = %s", maxHosts)

		for i := 0; i < len(targets) && i < maxHosts; i++ {
			logger.Debug("resolveRequestTargets: target = %s", *targets[i])
			out <- targets[i]
		}
		logger.Debug("resolveRequestTargets: Goroutine returning")
	}()

	return out, nil
}

func (r *Runner) dispatch(ctx context.Context, check *Check, targets chan *Target) (chan *Task, error) {
	// If the Check submitted is invalid, RunCheck will return a single
	// CheckResponse indicating that there was an error with the Check.
	c, err := UnmarshalAny(check.CheckSpec)
	if err != nil {
		logger.Error("dispatch - unable to unmarshal check: %s", err.Error())
		return nil, err
	}
	logger.Debug("dispatch - check = %s", check)

	tg := TaskGroup{}

	for target := range targets {
		if target != nil {
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
	}

	return r.dispatcher.Dispatch(ctx, tg), nil
}

func (r *Runner) runCheck(ctx context.Context, check *Check, responses chan *CheckResponse) {
	// If there is an error resolving the target, RunCheck will return a single
	// CheckResponse indicating that there was a target resolution error.
	targets, err := r.resolveRequestTargets(ctx, check)
	if err != nil {
		responses <- &CheckResponse{
			Target: check.Target,
			Error:  err.Error(),
		}
		return
	}

	finishedTasks, err := r.dispatch(ctx, check, targets)
	if err != nil {
		responses <- &CheckResponse{
			Target: check.Target,
			Error:  err.Error(),
		}
	}

	for t := range finishedTasks {
		logger.Debug("runCheck - Handling finished task: %s", *t)
		var responseError string
		var responseAny *Any
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
// If there is an error resolving the target, RunCheck will return a single
// CheckResponse indicating that there was a target resolution error.
//
// If the Check submitted is invalid, RunCheck will return a single
// CheckResponse indicating that there was an error with the Check.
//
// If the Context passed to RunCheck includes a MaxHosts value, at most MaxHosts
// CheckResponse objects will be returned.
//
// If the Context passed to RunCheck is cancelled or its deadline is exceeded,
// all CheckResponse objects after that event will be passed to the channel
// with appropriate errors associated with them.
func (r *Runner) RunCheck(ctx context.Context, check *Check) chan *CheckResponse {
	responses := make(chan *CheckResponse, 1)
	go func() {
		defer close(responses)
		r.runCheck(ctx, check, responses)
	}()
	return responses
}
