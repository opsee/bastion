package messaging

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bitly/go-nsq"
)

type Producer interface {
	Publish(message interface{}) error
	PublishRepliable(id string, msg EventInterface) error
	Close() error
}

type NsqProducer struct {
	Topic      string
	CustomerId string

	nsqProducer *nsq.Producer
	nsqConfig   *nsq.Config
}

// NewProducer will create a named channel on the specified topic and return
// a Producer attached to a channel.
func NewProducer(topicName string) (Producer, error) {
	producer := &NsqProducer{
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

func NewCustomerProducer(customerId, topicName string) (Producer, error) {
	producer, err := NewProducer(topicName)
	if err != nil {
		return nil, err
	}

	producer.(*NsqProducer).CustomerId = customerId
	return producer, nil
}

// Publish synchronously sends a message to the producer's given topic.
func (p *NsqProducer) Publish(message interface{}) error {
	event, err := NewEvent(message)
	if err != nil {
		logger.Error(err.Error())
	}

	if p.CustomerId != "" {
		event.CustomerId = p.CustomerId
	}

	eBytes, _ := json.Marshal(event)

	logger.Info("Publishing event: %s", string(eBytes))
	return p.nsqProducer.Publish(p.Topic, eBytes)
}

func (p *NsqProducer) PublishRepliable(id string, msg EventInterface) error {
	event, _ := NewEvent(msg)
	event.MessageId = id

	if p.CustomerId != "" {
		event.CustomerId = p.CustomerId
	}

	eBytes, _ := json.Marshal(event)

	return p.nsqProducer.Publish(p.Topic, eBytes)
}

// Stop gracefully terminates producing to nsqd.
// NOTE: This blocks until completion.
func (p *NsqProducer) Close() error {
	errChan := make(chan error, 1)
	go func(errChan chan error) {
		p.nsqProducer.Stop()
		errChan <- nil
	}(errChan)
	select {
	case err := <-errChan:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("Timed out waiting for producer to stop.")
	}
}
