package monitor

import (
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"github.com/nsqio/go-nsq"
	"github.com/opsee/bastion/heart"
)

const (
	moduleName = "monitor"
)

var (
	components = []string{
		"runner",
		"test_runner",
		"checker",
		"discovery",
		"monitor",
	}
)

type Monitor struct {
	components map[string]*Component
	consumer   *nsq.Consumer
	statemap   map[string]*State
}

func NewMonitor(nsqdHost string) (*Monitor, error) {
	nsqConfig := nsq.NewConfig()
	nsqConfig.ClientID = "monitor"

	consumer, err := nsq.NewConsumer(heart.Topic, moduleName, nsqConfig)
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

	consumer.AddHandler(nsq.HandlerFunc(func(msg *nsq.Message) error {
		log.WithFields(log.Fields{"service": "monitor"}).Debug("monitor event received")

		heartBeat := new(heart.HeartBeat)

		if err := json.Unmarshal(msg.Body, heartBeat); err != nil {
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

		return nil
	}))

	if err := consumer.ConnectToNSQD(nsqdHost); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Monitor) Stop() {
	m.consumer.Stop()
	<-m.consumer.StopChan
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
