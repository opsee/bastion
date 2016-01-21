package monitor

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/messaging"
)

const (
	moduleName = "monitor"
)

var (
	components = []string{
		"runner",
		"aws_command",
		"checker",
		"discovery",
	}
)

type Monitor struct {
	components map[string]*Component
	consumer   messaging.Consumer
	statemap   map[string]*State
}

func NewMonitor() (*Monitor, error) {
	consumer, err := messaging.NewConsumer(heart.Topic, moduleName)
	if err != nil {
		return nil, err
	}

	m := &Monitor{
		consumer:   consumer,
		components: make(map[string]*Component),
		statemap:   make(map[string]*State),
	}

	for _, c := range components {
		cm := NewComponent(c)
		m.components[c] = cm
		m.statemap[c] = cm.State
	}

	go m.monitorState()

	return m, nil
}

func (m *Monitor) monitorState() {
	for event := range m.consumer.Channel() {
		log.WithFields(log.Fields{"service": "monitor"}).Debug("monitor event received")

		heartBeat := new(heart.HeartBeat)

		if err := json.Unmarshal([]byte(event.Body()), heartBeat); err != nil {
			log.WithFields(log.Fields{"service": "monitor"}).Error("monitor failed to unmarshall event")
		} else {
			log.WithFields(log.Fields{"service": "monitor", "component": heartBeat.Process, "heartbeat timestamp": heartBeat.Timestamp}).Debug("monitor unmarshalled event")

			// ensure we are monitoring this component
			if component, ok := m.components[heartBeat.Process]; ok {
				component.Send(heartBeat)
			} else {
				log.WithFields(log.Fields{"service": "monitor", "component": heartBeat.Process, "heartbeat timestamp": heartBeat.Timestamp}).Warning("monitor unmarshalled event from unmonitored component")
			}
		}

		event.Ack()
	}
}

func (m *Monitor) SerializeState() ([]byte, error) {
	log.Debug("SerializeState")
	jsonBytes, err := json.Marshal(m.statemap)
	if err != nil {
		return nil, err
	}
	log.Debug("Serialize Monitor State Map: ", jsonBytes)
	return jsonBytes, nil
}
