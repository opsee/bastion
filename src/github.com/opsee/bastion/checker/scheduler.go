package checker

import (
	"fmt"
	"golang.org/x/net/context"
	"reflect"
	"sync"
	"time"
)

const (
	// Checks with an interval less than 15 seconds will fail to be created.
	MinimumCheckInterval = 15
)

func validateCheck(check *Check) error {
	if check.Id == "" {
		return fmt.Errorf("Check has null ID")
	}
	if check.Interval < MinimumCheckInterval {
		return fmt.Errorf("Check interval below threshold (%d minimum): %d", MinimumCheckInterval, check.Interval)
	}
	if check.Target == nil {
		return fmt.Errorf("Check has null target")
	}
	if check.CheckSpec == nil {
		return fmt.Errorf("Check has null CheckSpec")
	}

	return nil
}

/*******************************************************************************
 * check with ticker
 ******************************************************************************/

type checkWithTicker struct {
	Check  *Check
	stop   chan struct{}
	ticker *time.Ticker
}

func newCheckWithTicker(check *Check) (*checkWithTicker, error) {
	d, err := time.ParseDuration(fmt.Sprintf("%ds", check.Interval))
	if err != nil {
		return nil, err
	}
	ct := &checkWithTicker{
		check,
		make(chan struct{}, 1),
		time.NewTicker(d),
	}

	return ct, nil
}

func (c *checkWithTicker) Stop() {
	c.ticker.Stop()
	c.stop <- struct{}{}
}

/*******************************************************************************
 * schedule map
 ******************************************************************************/

type scheduleMap struct {
	sync.RWMutex
	checks map[string]*checkWithTicker
}

func newScheduleMap() *scheduleMap {
	return &scheduleMap{
		checks: make(map[string]*checkWithTicker),
	}
}

func (m *scheduleMap) Set(key string, check *Check) (*checkWithTicker, error) {
	m.Lock()
	defer m.Unlock()
	ct, err := newCheckWithTicker(check)
	if err != nil {
		return nil, err
	}
	m.checks[key] = ct

	return ct, nil
}

func (m *scheduleMap) Get(key string) *checkWithTicker {
	m.RLock()
	v := m.checks[key]
	m.RUnlock()
	return v
}

func (m *scheduleMap) Delete(key string) *checkWithTicker {
	m.Lock()
	v := m.checks[key]
	delete(m.checks, key)
	m.Unlock()

	return v
}

//
//  Scheduler
//
type Scheduler struct {
	scheduleMap *scheduleMap
	dispatcher  *Dispatcher
	Resolver    Resolver
}

func NewScheduler() *Scheduler {
	scheduler := &Scheduler{
		scheduleMap: newScheduleMap(),
		dispatcher:  NewDispatcher(),
	}

	scheduler.dispatcher.Dispatch()

	return scheduler
}

func (s *Scheduler) resolveRequestTargets(ctx context.Context, errors chan error, check *Check) chan *string {
	out := make(chan *string)

	go func() {
		defer close(out)

		targets, err := s.Resolver.Resolve(check.Target)
		if err != nil {
			errors <- err
			return
		}

		if len(targets) == 0 {
			errors <- fmt.Errorf("No valid targets resolved from %s", check.Target)
			return
		}

		var (
			maxHosts int
			ok       bool
		)

		maxHosts, ok = ctx.Value("MaxHosts").(int)
		if !ok {
			maxHosts = len(targets)
		}
		logger.Debug("resolveRequestTargets: MaxHosts = %s", maxHosts)

		for i := 0; i < int(maxHosts) && i < len(targets); i++ {
			logger.Debug("resolveRequestTargets: target = %s", *targets[i])
			out <- targets[i]
		}
		logger.Debug("resolveRequestTargets: Goroutine returning")
	}()

	return out
}

func (s *Scheduler) makeRequestFromCheck(ctx context.Context, errors chan error, check *Check, targets chan *string) chan Request {
	requests := make(chan Request)

	go func(check *Check) {
		defer close(requests)

		c, err := UnmarshalAny(check.CheckSpec)
		if err != nil {
			logger.Error("makeRequestFromCheck - unable to unmarshal check: %s", err.Error())
			errors <- err
			return
		}
		logger.Debug("makeRequestFromCheck - check = %s", check)

		for {
			select {
			case target, ok := <-targets:
				if !ok {
					logger.Debug("makeRequestFromCheck - targets channel closed")
					return
				}
				if target != nil {
					logger.Debug("makeRequestFromCheck - Handling target: %s", *target)
					switch typedCheck := c.(type) {
					case *HttpCheck:
						uri := fmt.Sprintf("%s://%s:%d%s", typedCheck.Protocol, *target, typedCheck.Port, typedCheck.Path)
						request := &HTTPRequest{
							Method:  typedCheck.Verb,
							URL:     uri,
							Headers: typedCheck.Headers,
							Body:    typedCheck.Body,
						}
						requests <- request
					default:
						logger.Error("makeRequestFromCheck - Unknown check type: %s", reflect.TypeOf(c))
						errors <- err
						return
					}
				}
			case <-ctx.Done():
				logger.Error("makeRequestFromCheck - %s", ctx.Err())
				return
			}
		}
		logger.Debug("makeRequestFromCheck - goroutine returned")
	}(check)

	return requests
}

func (s *Scheduler) makeTasksFromRequests(ctx context.Context, errors chan error, requests chan Request) chan *Task {
	tasks := make(chan *Task)

	go func() {
		defer close(tasks)

		for {
			select {
			case req := <-requests:
				if req != nil {
					logger.Debug("makeTasksFromRequests - handling request: %s", req)
					t := reflect.TypeOf(req).Elem().Name()
					task := &Task{
						Type:    t,
						Request: req,
					}
					tasks <- task
				}
			case <-ctx.Done():
				logger.Debug("makeTasksFromRequests - %s", ctx.Err())
				return
			}
		}
		logger.Debug("makeTasksFromRequests - goroutine returning")
	}()
	return tasks
}

func (s *Scheduler) dispatchTasks(ctx context.Context, errors chan error, tasks chan *Task, responses chan Response) {
	go func() {
		defer close(responses)

		for {
			select {
			case t := <-tasks:
				if t != nil {
					logger.Debug("dispatchTasks - dispatching task: %s", t)
					t.Response = responses
					s.dispatcher.Tasks <- t
				}
			case <-ctx.Done():
				logger.Debug("dispatchTasks - %s", ctx.Err())
				return
			}
		}
	}()
}

func (s *Scheduler) RunCheck(ctx context.Context, check *Check) (chan Response, chan error) {
	responses := make(chan Response)
	errors := make(chan error, 1)

	go func() {
		targets := s.resolveRequestTargets(ctx, errors, check)
		requests := s.makeRequestFromCheck(ctx, errors, check, targets)
		tasks := s.makeTasksFromRequests(ctx, errors, requests)
		s.dispatchTasks(ctx, errors, tasks, responses)
	}()

	return responses, errors
}

// CreateCheck takes as its input a Check. It maintains an internal mapping
// of check.ID -> check. If a check for that ID already exists, it will return
// the previous value for the Check. This is so that we can be aware of check
// redefinition when it happens.
func (s *Scheduler) CreateCheck(check *Check) (*Check, error) {
	if err := validateCheck(check); err != nil {
		return check, err
	}

	ct, err := s.scheduleMap.Set(check.Id, check)
	if err != nil {
		return nil, err
	}

	return ct.Check, nil
}

// Retrieve a Check by ID. If a check associated with the ID exists, then it
// will be returned. Otherwise, it will return nil and an error indicating the
// check does not exist.
func (s *Scheduler) RetrieveCheck(id string) (*Check, error) {
	var (
		ct  *checkWithTicker
		err error
	)

	ct = s.scheduleMap.Get(id)

	if ct == nil {
		err = fmt.Errorf("Non-existent check: %s", id)
		return nil, err
	}

	return ct.Check, err
}

func (s *Scheduler) DeleteCheck(id string) (*Check, error) {
	var (
		c   *checkWithTicker
		err error
	)
	c = s.scheduleMap.Delete(id)

	if c == nil {
		err = fmt.Errorf("Non-existent check: %s", id)
		return nil, err
	}
	c.Stop()

	return c.Check, err
}
