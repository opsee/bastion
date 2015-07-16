package messaging

import (
	"encoding/json"
	"fmt"

	"github.com/bitly/go-nsq"
)

type Producer struct {
	Topic string

	nsqProducer *nsq.Producer
	nsqConfig   *nsq.Config
}

// NewProducer will create a named channel on the specified topic and return
// a Producer attached to a channel.
func NewProducer(topicName string) (*Producer, error) {
	producer := &Producer{
		Topic:     topicName,
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
	event, err := NewEvent(message)
	if err != nil {
		logger.Error(err.Error())
	}

	eBytes, _ := json.Marshal(event)

	logger.Info("Publishing event: %s", string(eBytes))
	return p.nsqProducer.Publish(p.Topic, eBytes)
}

func (p *Producer) PublishRepliable(id string, msg EventInterface) error {
	event, _ := NewEvent(msg)
	event.MessageId = id
	eBytes, _ := json.Marshal(event)

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
