package messaging

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/bitly/go-nsq"
	"github.com/opsee/bastion/netutil"
)

// A Producer is a tuple of a Topic and a Channel.
type Producer struct {
	Topic      string
	RoutingKey string
	Channel    chan *netutil.Event

	nsqProducer *nsq.Producer
	nsqConfig   *nsq.Config
}

// NewProducer will create a named channel on the specified topic and return
// a Producer attached to a channel.
func NewProducer(topicName string) (*Producer, error) {
	channel := make(chan *netutil.Event)

	producer := &Producer{
		Topic:     topicName,
		Channel:   channel,
		nsqConfig: nsq.NewConfig(),
	}

	nsqProducer, err := nsq.NewProducer(getNsqdURL(), producer.nsqConfig)

	if err != nil {
		return nil, fmt.Errorf("Error creating producer: %v", err)
	}

	producer.nsqProducer = nsqProducer

	return producer, nil
}

// Publish synchronously sends a message to the producer's given topic.
func (p *Producer) Publish(message interface{}) error {
	msgBytes, err := json.Marshal(message)
	if err != nil {
		logger.Error("%s", err)
	}

	event := &netutil.Event{
		Type: reflect.ValueOf(message).Elem().Type().Name(),
		Body: string(msgBytes),
	}

	eBytes, _ := json.Marshal(event)
	logger.Debug("Publishing event: %s", string(eBytes))
	return p.nsqProducer.Publish(p.Topic, eBytes)
}

// Start the Producer
func (p *Producer) Start() error {
	return nil
}

// Stop gracefully terminates producing to nsqd.
// NOTE: This blocks until completion.
func (p *Producer) Stop() {
	p.nsqProducer.Stop()
}
