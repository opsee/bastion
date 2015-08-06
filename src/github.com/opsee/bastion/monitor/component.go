package monitor

import (
	"time"

	"github.com/opsee/bastion/heart"
)

const (
	HeartBeatTimeout       = 30 * time.Second
	HeartBeatCheckInterval = 5 * time.Second
)

type State struct {
	OK        bool `json:"ok"`
	HeartBeat *heart.HeartBeat
}

// Component is a basic statemachine
// Heartbeat timeout: OK(true)->OK(false) ; OK(false)->OK(false)
// Heartbeat received: OK(true)->OK(true) ; OK(false)->OK(true)
type Component struct {
	ticker           *time.Ticker
	State            *State
	HeartBeatChannel chan *heart.HeartBeat
}

func NewComponent(name string) *Component {
	c := &Component{
		ticker:           time.NewTicker(HeartBeatCheckInterval),
		State:            &State{},
		HeartBeatChannel: make(chan *heart.HeartBeat, 1),
	}

	go c.loop()

	return c
}

func (c *Component) Send(hb *heart.HeartBeat) {
	c.HeartBeatChannel <- hb
}

func (c *Component) loop() {
	for {
		select {
		case t := <-c.ticker.C:
			if c.State.HeartBeat != nil {
				timeoutTime := time.Unix(0, c.State.HeartBeat.Timestamp).Add(HeartBeatTimeout)
				if t.After(timeoutTime) {
					c.State.OK = false
				}
			} else {
				c.State.OK = false
			}
		case hb := <-c.HeartBeatChannel:
			c.State.HeartBeat = hb
			c.State.OK = true
		}
	}
}
