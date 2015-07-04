package messaging

import (
	"fmt"
	"time"

	"github.com/bitly/go-nsq"
)

// A Consumer is a triple of a Topic, RoutingKey, and a Channel.
type Consumer struct {
	Topic      string
	RoutingKey string
	Channel    chan string

	nsqConsumer *nsq.Consumer
	nsqConfig   *nsq.Config
}

// NewConsumer will create a named channel on the specified topic and return
// the associated message-producing channel.
func NewConsumer(topicName string, routingKey string) (*Consumer, error) {
	channel := make(chan string)

	consumer := &Consumer{
		Topic:      topicName,
		RoutingKey: routingKey,
		Channel:    channel,
		nsqConfig:  nsq.NewConfig(),
	}

	nsqConsumer, err := nsq.NewConsumer(topicName, routingKey, consumer.nsqConfig)
	if err != nil {
		return nil, err
	}

	consumer.nsqConsumer = nsqConsumer

	nsqConsumer.AddHandler(nsq.HandlerFunc(
		func(message *nsq.Message) error {
			body := string(message.Body)
			channel <- body
			return nil
		}))

	return consumer, nil
}

// Start the Consumer
func (c *Consumer) Start() error {
	var err error

	url := getNsqLookupdURL()
	if err := c.nsqConsumer.ConnectToNSQD(url); err != nil {
		err = fmt.Errorf("Unable to connect to NSQLookupd at %s", url)
	}

	return err
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

	close(c.Channel)
	return err
}
