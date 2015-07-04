package messaging

import (
	"fmt"

	"github.com/bitly/go-nsq"
)

// A Producer is a tuple of a Topic and a Channel.
type Producer struct {
	Topic      string
	RoutingKey string
	Channel    chan string

	nsqProducer *nsq.Producer
	nsqConfig   *nsq.Config
}

// NewProducer will create a named channel on the specified topic and return
// a Producer attached to a channel.
func NewProducer(topicName string) (*Producer, error) {
	channel := make(chan string)

	producer := &Producer{
		Topic:     topicName,
		Channel:   channel,
		nsqConfig: nsq.NewConfig(),
	}

	return producer, nil
}

// Publish synchronously sends a message to the producer's given topic.
func (p *Producer) Publish(message string) error {
	return p.nsqProducer.Publish(p.Topic, []byte(message))
}

// Start the Producer
func (p *Producer) Start() error {
	nsqProducer, err := nsq.NewProducer(getNsqLookupdURL(), p.nsqConfig)

	if err != nil {
		return fmt.Errorf("Error creating producer: %v", err)
	}

	p.nsqProducer = nsqProducer
	return nil
}

// Stop gracefully terminates producing to nsqd.
// NOTE: This blocks until completion.
func (p *Producer) Stop() {
	p.nsqProducer.Stop()
}
