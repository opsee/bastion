package checker

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
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
	Check   *Check
	runChan chan *Check
	stop    chan struct{}
	ticker  *time.Ticker
}

func newCheckWithTicker(check *Check, runChan chan *Check) (*checkWithTicker, error) {
	d, err := time.ParseDuration(fmt.Sprintf("%ds", check.Interval))
	if err != nil {
		return nil, err
	}
	ct := &checkWithTicker{
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

func (c *checkWithTicker) Stop() {
	c.stop <- struct{}{}
}

/*******************************************************************************
 * schedule map
 ******************************************************************************/

type scheduleMap struct {
	sync.RWMutex
	checks  map[string]*checkWithTicker
	runChan chan *Check
}

func newScheduleMap() *scheduleMap {
	return &scheduleMap{
		checks:  make(map[string]*checkWithTicker),
		runChan: make(chan *Check, 10),
	}
}

func (m *scheduleMap) RunChan() chan *Check {
	return m.runChan
}

func (m *scheduleMap) Set(key string, check *Check) (*checkWithTicker, error) {
	m.Lock()
	defer m.Unlock()
	ct, err := newCheckWithTicker(check, m.runChan)
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

// Destroy will stop all of the tickers in a schedulemap and close the
// channel returned by RunChan().
func (m *scheduleMap) Destroy() {
	m.Lock()
	for _, check := range m.checks {
		check.ticker.Stop()
	}
	close(m.runChan)
}

type Publisher interface {
	Publish(string, []byte) error
	Stop()
}

//
//  Scheduler
//
type Scheduler struct {
	scheduleMap *scheduleMap
	Producer    Publisher
	stopChan    chan struct{}
}

func NewScheduler() *Scheduler {
	scheduler := &Scheduler{
		scheduleMap: newScheduleMap(),
		stopChan:    make(chan struct{}, 1),
	}

	return scheduler
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
				if msg, err := proto.Marshal(check); err != nil {
					logger.Error(err.Error())

				} else {
					if err := s.Producer.Publish("checks", msg); err != nil {
						logger.Error(err.Error())
					} else {
						logger.Info("Scheduled check for execution: %s", check.Id)
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
