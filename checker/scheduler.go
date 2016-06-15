package checker

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/basic/schema"
)

const (
	// Checks with an interval less than 15 seconds will fail to be created.
	MinimumCheckInterval = 15
)

func validateCheck(check *schema.Check) error {
	if check.Id == "" {
		return fmt.Errorf("Check has null ID")
	}
	if check.Interval < MinimumCheckInterval {
		return fmt.Errorf("Check interval below threshold (%d minimum): %d", MinimumCheckInterval, check.Interval)
	}
	if check.Target == nil {
		return fmt.Errorf("Check has null target")
	}
	if check.Spec == nil {
		return fmt.Errorf("Check has null Spec")
	}

	return nil
}

// CheckTimer sends a check over a channel at a set interval.
// TODO(greg): Instead of sending check pointers over this channel, we should send a check execution
// task -- some wrapper object with a context that includes a deadline. Basically, add contexts to
// scheduled executions.
type CheckTimer struct {
	Check   *schema.Check
	runChan chan *schema.Check
	stop    chan struct{}
	ticker  *time.Ticker
}

// NewCheckTimer creates a new timer and associates the given channel with that timer.
// Every N seconds (the Check's Duration field), the CheckTimer will send the check over
// the runChan channel. If this channel blocks for more than N seconds, you will start
// executing stale check requests. Managing backpressure for this channel is important.
// TODO(greg): manage backpressure for this channel.
func NewCheckTimer(check *schema.Check, runChan chan *schema.Check) (*CheckTimer, error) {
	d, err := time.ParseDuration(fmt.Sprintf("%ds", check.Interval))
	if err != nil {
		return nil, err
	}
	ct := &CheckTimer{
		check,
		runChan,
		make(chan struct{}, 1),
		time.NewTicker(d),
	}

	go func() {
		for {
			select {
			case <-ct.ticker.C:
				ct.runChan <- ct.Check
			case <-ct.stop:
				ct.ticker.Stop()
				return
			}
		}
	}()

	return ct, nil
}

// Stop the Check's timer.
func (c *CheckTimer) Stop() {
	c.stop <- struct{}{}
}

/*******************************************************************************
 * schedule map
 ******************************************************************************/

type scheduleMap struct {
	sync.RWMutex
	checks  map[string]*CheckTimer
	runChan chan *schema.Check
}

func newScheduleMap() *scheduleMap {
	return &scheduleMap{
		checks:  make(map[string]*CheckTimer),
		runChan: make(chan *schema.Check, 10),
	}
}

func (m *scheduleMap) RunChan() chan *schema.Check {
	return m.runChan
}

// Set adds a new CheckTimer to the schedule map, returning the CheckTimer
// after creation. It blocks acquiring a write lock on the schedule map.

func (m *scheduleMap) Set(key string, check *schema.Check) (*CheckTimer, error) {
	m.Lock()
	defer m.Unlock()
	ct, err := NewCheckTimer(check, m.runChan)
	if err != nil {
		return nil, err
	}
	m.checks[key] = ct

	m.runChan <- ct.Check

	return ct, nil
}

// Get blocks until it can acquire a read lock on the schedule map, it then
// returns the CheckTimer associated with the requested CheckID.

func (m *scheduleMap) Get(key string) *CheckTimer {
	m.RLock()
	defer m.RUnlock()
	v := m.checks[key]
	return v
}

// Delete blocks until it can acquire a write lock on the schedule map, and
// then deletes the check from the schedule map. It also stops the ticker for
// the check so that it can be GC'd.

func (m *scheduleMap) Delete(key string) *CheckTimer {
	m.Lock()
	defer m.Unlock()
	v := m.checks[key]
	if v != nil {
		v.Stop()
	}
	delete(m.checks, key)

	return v
}

// Destroy will stop all of the tickers in a schedulemap and close the
// channel returned by RunChan().
func (m *scheduleMap) Destroy() {
	m.Lock()
	defer m.Unlock()
	for _, check := range m.checks {
		check.ticker.Stop()
	}
	close(m.runChan)
}

type Publisher interface {
	Publish(string, []byte) error
	Stop()
}

//  Scheduler is responsible for managing the set of timers used for checks
// as well as publishing requests for runners to run checks.
type Scheduler struct {
	scheduleMap *scheduleMap
	Producer    Publisher
	stopChan    chan struct{}
	resolver    Resolver
}

// NewScheduler creates a funcitoning scheduler including its own scheduleMap.

func NewScheduler(r Resolver) *Scheduler {
	scheduler := &Scheduler{
		scheduleMap: newScheduleMap(),
		stopChan:    make(chan struct{}, 1),
		resolver:    r,
	}

	return scheduler
}

// CreateCheck takes as its input a Check. It maintains an internal mapping of
// check.ID -> check. If a check for that ID already exists, it will return the
// previous value for the Check. This is so that we can be aware of check
// redefinition when it happens.

func (s *Scheduler) CreateCheck(check *schema.Check) (*schema.Check, error) {
	// since we're still retrieving checks from bartnet, turn the check spec into the oneof
	// this should be the sole entrypoint for checks
	// TODO: remove this after no bartnet
	if check.Spec == nil {
		any, err := opsee_types.UnmarshalAny(check.CheckSpec)
		if err != nil {
			log.WithError(err).Error("couldn't unmarshal the check spec from bartnet")
			return nil, err
		}

		switch spec := any.(type) {
		case *schema.HttpCheck:
			check.Spec = &schema.Check_HttpCheck{spec}
		case *schema.CloudWatchCheck:
			check.Spec = &schema.Check_CloudwatchCheck{spec}
		}
	}

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

func (s *Scheduler) RetrieveCheck(check *schema.Check) (*schema.Check, error) {
	var (
		ct  *CheckTimer
		err error
	)

	ct = s.scheduleMap.Get(check.Id)

	if ct == nil {
		err = fmt.Errorf("Non-existent check: %s", check.Id)
		return nil, err
	}

	return ct.Check, err
}

func (s *Scheduler) DeleteCheck(check *schema.Check) (*schema.Check, error) {
	var (
		c   *CheckTimer
		err error
	)
	c = s.scheduleMap.Delete(check.Id)

	if c == nil {
		err = fmt.Errorf("Non-existent check: %s", check.Id)
		return nil, err
	}
	c.Stop()

	return c.Check, err
}

// Start scheduling checks from this Scheduler's ScheduleMap

func (s *Scheduler) Start() error {
	if s.Producer == nil {
		return fmt.Errorf("Scheduler must have a Publisher.")
	}

	go func() {
		for {
			select {
			case <-s.stopChan:
				s.Producer.Stop()
				s.scheduleMap.Destroy()
				return
			case check := <-s.scheduleMap.RunChan():
				if checkWithTargets, err := NewCheckTargets(s.resolver, check); err != nil {
					log.Error(err.Error())
				} else {
					jsonBytes, err := json.Marshal(checkWithTargets)
					if err != nil {
						log.Error(err.Error())
					} else {
						// TODO(greg): All of the channel configuration stuff, really needs to
						// be centralized and easily managed. It can just be a static file or
						// something that every microservice refers to--just to make sure
						// they're all on the same page.
						if err := s.Producer.Publish("runner", jsonBytes); err != nil {
							log.Error(err.Error())
						} else {
							log.Debug("Scheduled check for execution: %s", check.Id)
						}
					}
				}
			}
		}
	}()

	return nil
}

func (s *Scheduler) Stop() {
	s.stopChan <- struct{}{}
}
