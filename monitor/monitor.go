package monitor

import (
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
)

const (
	moduleName = "monitor"
)

var (
	logger = logging.GetLogger(moduleName)
)

type State map[string]*ComponentState
type ComponentState struct {
	Metrics map[string]uint64
}

type Monitor struct {
	State    State
	consumer messaging.Consumer
}

func NewMonitor() (*Monitor, error) {
	consumer, err := messaging.NewConsumer(heart.Topic, moduleName)
	if err != nil {
		return nil, err
	}

	m := &Monitor{
		consumer: consumer,
		State:    State{},
	}

	go m.monitorState()

	return m, nil
}

func (m *Monitor) monitorState() {
	for event := range m.consumer.Channel() {
		logger.Debug("event received: %s", event)
		heartBeat := new(heart.HeartBeat)
		if m.State[heartBeat.Process] != nil {
			m.State[heartBeat.Process] = &ComponentState{}
		}

		m.State[heartBeat.Process].Metrics = heartBeat.Metrics

		event.Ack()
	}
}

func (m *Monitor) Healthy() bool {
	return true
}
