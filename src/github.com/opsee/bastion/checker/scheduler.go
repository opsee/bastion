package checker

import (
	"fmt"
	"sync"
)

const (
	MinimumCheckInterval = 15
)

type concurrentMap struct {
	sync.RWMutex
	checks map[string]*Check
}

func (m *concurrentMap) Set(key string, check *Check) {
	m.Lock()
	m.checks[key] = check
	m.Unlock()
}

func (m *concurrentMap) Get(key string) *Check {
	m.RLock()
	v := m.checks[key]
	m.RUnlock()
	return v
}

func (m *concurrentMap) Delete(key string) *Check {
	m.Lock()
	v := m.checks[key]
	delete(m.checks, key)
	m.Unlock()
	return v
}

type Scheduler struct {
	checkMap *concurrentMap
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		checkMap: &concurrentMap{
			checks: make(map[string]*Check),
		},
	}
}

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

func (s *Scheduler) CreateCheck(check *Check) (*Check, error) {
	if err := validateCheck(check); err != nil {
		return check, err
	}

	s.checkMap.Set(check.Id, check)
	return s.checkMap.Get(check.Id), nil
}

func (s *Scheduler) RetrieveCheck(id string) (*Check, error) {
	var (
		c   *Check
		err error
	)

	c = s.checkMap.Get(id)

	if c == nil {
		err = fmt.Errorf("Non-existent check: %s", id)
	}

	return c, err
}

func (s *Scheduler) DeleteCheck(id string) (*Check, error) {
	var (
		c   *Check
		err error
	)
	c = s.checkMap.Delete(id)

	if c == nil {
		err = fmt.Errorf("Non-existent check: %s", id)
	}

	return c, err
}
