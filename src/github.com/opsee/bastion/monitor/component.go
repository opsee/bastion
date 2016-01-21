package monitor

import (
	"time"

	log "github.com/Sirupsen/logrus"
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
	Name             string
}

func NewComponent(name string) *Component {
	c := &Component{
		ticker:           time.NewTicker(HeartBeatCheckInterval),
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

func (c *Component) loop() {
	for {
		select {

		case t := <-c.ticker.C:
			// if c.State.HeartBeat was an error (not from HeartBeatChannel)
			if c.State.HeartBeat != nil {

				// create a timestamp form
				timeoutTime := time.Unix(0, c.State.HeartBeat.Timestamp).Add(HeartBeatTimeout)

				if t.After(timeoutTime) {
					c.State.OK = false
					log.WithFields(log.Fields{"component": c.Name, "heartbeat": nil, "timoutTime": timeoutTime, "state OK": c.State.OK}).Warning("component timed out.")
				}
			} else {
				c.State.OK = false
				log.WithFields(log.Fields{"component": c.Name, "heartbeat": nil, "state OK": c.State.OK}).Warning("component nil heartbeat.")
			}
		case hb := <-c.HeartBeatChannel:
			c.State.HeartBeat = hb
			c.State.OK = true
			log.WithFields(log.Fields{"component": c.Name, "heartbeat": c.State.HeartBeat, "state OK": c.State.OK}).Debug("component in good state")
		}
	}
}
