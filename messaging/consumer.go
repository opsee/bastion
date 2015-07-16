package messaging

import (
	"fmt"
	"time"

	"github.com/bitly/go-nsq"
)

type Consumer struct {
	Topic      string
	RoutingKey string

	channel     chan *Event
	nsqConsumer *nsq.Consumer
	nsqConfig   *nsq.Config
}

// NewConsumer will create a named channel on the specified topic and return
// the associated message-producing channel.
func NewConsumer(topicName string, routingKey string) (*Consumer, error) {
	channel := make(chan *Event, 1)

	consumer := &Consumer{
		Topic:      topicName,
		RoutingKey: routingKey,
		nsqConfig:  nsq.NewConfig(),
		channel:    channel,
	}

	nsqConsumer, err := nsq.NewConsumer(topicName, routingKey, consumer.nsqConfig)
	if err != nil {
		return nil, err
	}

	if replyProducer == nil {
		replyProducer, err = nsq.NewProducer(getNsqdURL(), consumer.nsqConfig)
		if err != nil {
			return nil, err
		}
	}

	nsqConsumer.AddHandler(nsq.HandlerFunc(
		func(message *nsq.Message) error {
			event, err := NewEvent(message)
			if err != nil {
				return err
			}

			channel <- event
			return nil
		}))

	nsqConsumer.ConnectToNSQD(getNsqdURL())

	return consumer, nil
}

func (c *Consumer) Channel() <-chan *Event {
	return c.channel
}

// Stop will first attempt to gracefully shutdown the Consumer. Failing
// to shutdown within a 5-second timeout, it closes channels and shuts down
// the consumer.
func (c *Consumer) Stop() error {
	c.nsqConsumer.Stop()

	var err error
	select {
	case <-c.nsqConsumer.StopChan:
		err = nil
	case <-time.After(5 * time.Second):
		err = fmt.Errorf("Timed out waiting for Consumer to stop.")
	}

	close(c.channel)
	return err
}
