package messaging

import (
	"fmt"
	"time"

	"github.com/bitly/go-nsq"
)

type Consumer interface {
	Channel() <-chan EventInterface
	Close() error
}

type NsqConsumer struct {
	Topic      string
	RoutingKey string

	channel     chan EventInterface
	nsqConsumer *nsq.Consumer
	nsqConfig   *nsq.Config
}

// NewConsumer will create a named channel on the specified topic and return
// the associated message-producing channel.
func NewConsumer(topicName, routingKey string) (Consumer, error) {
	channel := make(chan EventInterface, 1)

	consumer := &NsqConsumer{
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

// Channel provides an accessor to a channel that yields events from the
// message bus. Do not close this channel directly. Instead call the
// Close() method on the Consumer.
func (c *NsqConsumer) Channel() <-chan EventInterface {
	return c.channel
}

// Close will first attempt to gracefully shutdown the Consumer. Failing
// to shutdown within a 5-second timeout, it closes channels and shuts down
// the consumer.
func (c *NsqConsumer) Close() error {
	c.nsqConsumer.Stop()

	var err error
	timer := time.NewTimer(5 * time.Second)
	select {
	case <-c.nsqConsumer.StopChan:
		err = nil
	case <-timer.C:
		err = fmt.Errorf("Timed out waiting for Consumer to stop.")
	}

	timer.Stop()
	close(c.channel)
	return err
}
