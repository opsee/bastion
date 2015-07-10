package messaging

import (
	"fmt"
	"time"

	"github.com/bitly/go-nsq"
	"github.com/opsee/bastion/netutil"
)

type Consumer struct {
	Topic      string
	RoutingKey string

	channel     chan netutil.EventInterface
	nsqConsumer *nsq.Consumer
	nsqConfig   *nsq.Config
}

// NewConsumer will create a named channel on the specified topic and return
// the associated message-producing channel.
func NewConsumer(topicName string, routingKey string) (*Consumer, error) {
	channel := make(chan netutil.EventInterface, 1)

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

	consumer.nsqConsumer = nsqConsumer

	nsqConsumer.AddHandler(nsq.HandlerFunc(
		func(message *nsq.Message) error {
			event, err := netutil.NewEvent(message)
			if err != nil {
				return err
			}

			channel <- event
			return nil
		}))

	nsqConsumer.ConnectToNSQD(getNsqdURL())

	return consumer, nil
}

func (c *Consumer) Channel() <-chan netutil.EventInterface {
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
