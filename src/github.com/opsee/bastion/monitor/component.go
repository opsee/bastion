package monitor

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/bastion/heart"
)

const (
	// HeartBeatTimeout is the maximum age of a component's heartbeat.
	// After this time, the heartbeat is considered to be stale and the
	// component in a bad state.
	HeartBeatTimeout = 30 * time.Second

	// HeartBeatWaitTimeout is the maximum amount of time the component will
	// wait to get a heartbeat. After this time, the component will examine
	// the last heartbeat received to determine if it has timed out. If so,
	// the component will be set at a bad state.
	HeartBeatWaitTimeout = 60 * time.Second
)

type State struct {
	OK        bool `json:"ok"`
	HeartBeat *heart.HeartBeat
}

// Component is a basic statemachine
// Heartbeat timeout: OK(true)->OK(false) ; OK(false)->OK(false)
// Heartbeat received: OK(true)->OK(true) ; OK(false)->OK(true)
//
// Once a HeartBeat is received, a component has HeartBeatWaitTimeout until the next
// heartbeat is received. When that heartbeat is received, it will be checked for
// recency. If it has not timed out (i.e. the HeartBeat is < 30 seconds old, its state
// will be the state of the component. If the HeartBeat is stale, it will be discarded
// and the component will be in a bad state.
type Component struct {
	State            *State
	HeartBeatChannel chan *heart.HeartBeat
	Name             string
}

func NewComponent(name string) *Component {
	c := &Component{
		State:            &State{},
		HeartBeatChannel: make(chan *heart.HeartBeat, 1),
		Name:             name,
	}

	go c.loop()
	log.WithFields(log.Fields{"component": name}).Info("Started component ticker loop")

	return c
}

func (c *Component) Send(hb *heart.HeartBeat) {
	log.WithFields(log.Fields{"component": c.Name, "heartbeat": hb}).Debug("component send heartbeat to heartbeatchannel")
	c.HeartBeatChannel <- hb
}

// Wait for heartbeats. If we don't receive one every 60 seconds, then we set the state of the component
// to Not Ok.
func (c *Component) loop() {
	for {
		waiter := time.NewTimer(HeartBeatWaitTimeout)

		select {
		case <-waiter.C:
			c.State.OK = false
			log.WithFields(log.Fields{"component": c.Name, "heartbeat": nil, "state OK": c.State.OK}).Warning("Timeout waiting for heartbeat.")

		case hb := <-c.HeartBeatChannel:
			nsec := c.State.HeartBeat.Timestamp * int64(time.Nanosecond) / int64(time.Second)
			ctime := time.Unix(nsec, 0)
			timeoutTime := time.Now().Add(HeartBeatTimeout)

			if ctime.After(timeoutTime) {
				c.State.OK = false
				log.WithFields(log.Fields{
					"component":  c.Name,
					"heartbeat":  nil,
					"timoutTime": timeoutTime,
					"OK":         c.State.OK}).Warning("Component received stale heartbeat.")
			} else {
				c.State.HeartBeat = hb
				c.State.OK = true
				log.WithFields(log.Fields{
					"component": c.Name,
					"heartbeat": c.State.HeartBeat,
					"state OK":  c.State.OK}).Debug("Component received good heartbeat.")
			}
		}

		waiter.Stop()
	}
}
