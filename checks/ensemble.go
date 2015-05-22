package checks

import (
    "time"
)

// Ensemble provides a means for running multiple checks against a group of hosts
// and reporting back on the results of those checks.
type Ensemble struct {
    interval time.Duration
    hosts    map[string]bool
    checks   map[Check]bool
    timer    *time.Ticker
    closing  chan bool
    subs     map[hostCheck][]chan <- CheckResults
}

type hostCheck struct {
    host  string
    check Check
}

func NewEnsemble(hosts []string, checks []Check, interval time.Duration) *Ensemble {
    ensemble := &Ensemble{
        interval,
        make(map[string]bool),
        make(map[Check]bool),
        time.NewTicker(interval),
        make(chan bool),
        make(map[hostCheck][]chan <- CheckResults)}
    for _, host := range hosts {
        ensemble.hosts[host] = true
    }
    for _, check := range checks {
        ensemble.checks[check] = true
    }
    go ensemble.loop()
    return ensemble
}

func (e *Ensemble) loop() {
    for {
        select {
        case <-e.timer.C:
            e.runChecks()
        case <-e.closing:
            return
        }
    }
}

func (e *Ensemble) runChecks() {
    waiting := 0
    resultsChan := make(chan CheckResults, len(e.hosts))
    for host := range e.hosts {
        for check := range e.checks {
            check.RunCheck(host, resultsChan)
            waiting++
        }
    }
    for waiting > 0 {
        results := <-resultsChan
        waiting--
        e.routeResults(results)
    }
}

func (e *Ensemble) routeResults(results CheckResults) {
    if channels, ok := e.subs[hostCheck{results.Host, results.Check}]; ok {
        for _, ch := range channels {
            ch <- results
        }
    }
}

func (e *Ensemble) AddHost(host string) {
    e.hosts[host] = true
}

func (e *Ensemble) RemoveHost(host string) {
    delete(e.hosts, host)
}

func (e *Ensemble) Subscribe(host string, check Check) <-chan CheckResults {
    hc := hostCheck{host, check}
    _, ok := e.subs[hc]
    if !ok {
        e.subs[hc] = make([]chan <- CheckResults, 0, 10)
    }
    ch := make(chan CheckResults, 1)
    e.subs[hc] = append(e.subs[hc], ch)
    return ch
}

func (e *Ensemble) AddCheck(check Check) {
    e.checks[check] = true
}

func (e *Ensemble) RemoveCheck(check Check) {
    delete(e.checks, check)
}
