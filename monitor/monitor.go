package monitor

import (
	"encoding/json"

	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
)

const (
	moduleName = "monitor"
)

var (
	logger     = logging.GetLogger(moduleName)
	components = []string{
		"connector",
		"checker",
		"monitor",
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
		logger.Debug("event received: %s", event)
		heartBeat := new(heart.HeartBeat)
		if err := json.Unmarshal([]byte(event.Body()), heartBeat); err != nil {
			logger.Error("Unable to unmarshal heartbeat: %s", *heartBeat)
		} else {
			m.components[heartBeat.Process].Send(heartBeat)
		}
		event.Ack()
	}
}

func (m *Monitor) SerializeState() ([]byte, error) {
	jsonBytes, err := json.Marshal(m.statemap)
	if err != nil {
		return nil, err
	}

	return jsonBytes, nil
}
