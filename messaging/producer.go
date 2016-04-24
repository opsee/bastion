package messaging

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
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
		log.Error(err.Error())
	}

	event.CustomerId = p.CustomerId

	eBytes, _ := json.Marshal(event)

	log.Debug("Publishing event: %s", string(eBytes))
	return p.nsqProducer.Publish(p.Topic, eBytes)
}

func (p *NsqProducer) PublishRepliable(id string, msg EventInterface) error {
	event, _ := NewEvent(msg)
	event.MessageId = id
	event.CustomerId = p.CustomerId

	eBytes, _ := json.Marshal(event)

	return p.nsqProducer.Publish(p.Topic, eBytes)
}

// Stop gracefully terminates producing to nsqd.
func (p *NsqProducer) Close() error {
	done := make(chan struct{}, 1)
	go func() {
		p.nsqProducer.Stop()
		close(done)
	}()

	timer := time.NewTimer(5 * time.Second)
	defer func() { timer.Stop() }()

	select {
	case <-done:
		return nil
	case <-timer.C:
		return errors.New("Timed out waiting for Producer to stop.")
	}
}
