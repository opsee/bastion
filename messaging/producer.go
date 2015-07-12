package messaging

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/bitly/go-nsq"
)

// A Producer is a tuple of a Topic and a Channel.
type Producer struct {
	Topic      string
	RoutingKey string

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
	msgBytes, err := json.Marshal(message)
	logger.Info("Marshaled message to: ", string(msgBytes))
	if err != nil {
		logger.Error("%s", err)
	}

	event := &Event{
		MessageType: reflect.ValueOf(message).Elem().Type().Name(),
		MessageBody: string(msgBytes),
	}
	logger.Info("Built event: ", event)

	eBytes, _ := json.Marshal(event)

	logger.Info("Publishing event: %s", string(eBytes))
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
