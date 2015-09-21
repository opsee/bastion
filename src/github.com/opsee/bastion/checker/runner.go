package checker

import (
	"fmt"
	"reflect"

	"golang.org/x/net/context"
)

type Runner struct {
	resolver   Resolver
	dispatcher *Dispatcher
}

func NewRunner(resolver Resolver) *Runner {
	dispatcher := NewDispatcher()
	dispatcher.Dispatch()
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
	finished := make(chan *Task, 1)
	c, err := UnmarshalAny(check.CheckSpec)
	if err != nil {
		logger.Error("dispatch - unable to unmarshal check: %s", err.Error())
		return nil, err
	}
	logger.Debug("dispatch - check = %s", check)

	go func() {
		for {
			select {
			case target, ok := <-targets:
				if !ok {
					logger.Debug("dispatch - targets channel closed")
					return
				}
				if target != nil {
					logger.Debug("dispatch - Handling target: %s", *target)

					var request Request
					switch typedCheck := c.(type) {
					case *HttpCheck:
						// XXX: This is mildly jank. When you resolve instances (we should have instances
						// at this point), you get a Target with a type of "ip" back and the address is
						// stored in the Id field. This will have to be redone.
						ip, err := r.resolver.Resolve(target)
						logger.Debug("dispatch - Resolved target to ip: %s", ip[0].Id)
						if err != nil {
							logger.Error(err.Error())
						}
						uri := fmt.Sprintf("%s://%s:%d%s", typedCheck.Protocol, ip[0].Id, typedCheck.Port, typedCheck.Path)
						request = &HTTPRequest{
							Method:  typedCheck.Verb,
							URL:     uri,
							Headers: typedCheck.Headers,
							Body:    typedCheck.Body,
						}
					default:
						logger.Error("dispatch - Unknown check type: %s", reflect.TypeOf(c))
						return
					}

					logger.Debug("dispatch - Creating task from request: %s", request)
					t := reflect.TypeOf(request).Elem().Name()
					logger.Debug("dispatch - Request type: %s", t)

					task := &Task{
						Target:   target,
						Type:     t,
						Request:  request,
						Finished: finished,
					}

					logger.Debug("dispatch - Dispatching task: %s", *task)

					r.dispatcher.Tasks <- task
				}
			case <-ctx.Done():
				logger.Debug("dispatch - %s", ctx.Err())
				return
			}
		}
	}()

	return finished, nil
}

func (r *Runner) runCheck(ctx context.Context, check *Check, responses chan *CheckResponse) {
	targets, err := r.resolveRequestTargets(ctx, check)
	if err != nil {
		responses <- &CheckResponse{
			Target: check.Target,
			Error:  err.Error(),
		}
	}

	finishedTasks, err := r.dispatch(ctx, check, targets)
	defer close(finishedTasks)

	// If the deadline is exceeded, we may panic because we try to write to
	// a closed channel. Avoid this by recovering. We want to try to avoid
	// crashing on bad check data anyway.
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Recovered from panic in runCheck: %v", r)
		}
	}()

	if err != nil {
		responses <- &CheckResponse{
			Target: check.Target,
			Error:  err.Error(),
		}
	}

	for {
		select {
		case t := <-finishedTasks:
			if t != nil {
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
		case <-ctx.Done():
			logger.Debug("runCheck - %s", ctx.Err())
			return
		}
	}
}

func (r *Runner) RunCheck(ctx context.Context, check *Check) chan *CheckResponse {
	responses := make(chan *CheckResponse, 1)
	go func() {
		r.runCheck(ctx, check, responses)
	}()
	return responses
}
