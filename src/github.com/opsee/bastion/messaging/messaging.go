/* Messaging acts as a gateway between the Bastion and whatever is used
 * to communicate between Bastion components. It's responsibility is to provide
 * convenient interfaces to facilitate safe, structured message passing to
 * prevent lost/dead messages. All "wire-level" serialization/deserialization
 * should occur within the messaging package so that plugging different
 * messaging subsystems in (or ripping them out entirely) is easier.
 */
package messaging

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	log "github.com/sirupsen/logrus"
	"github.com/bitly/go-nsq"
)

const (
	ReplyTopic          = "replies"
	messageFormatString = "nsq.Message{ID: %s, Body: %s}"
)

var (
	replyProducer *nsq.Producer = nil
)

func getNsqdURL() string {
	nsqdURL := os.Getenv("NSQD_HOST")
	return nsqdURL
}

type EventInterface interface {
	Ack()
	Reply(reply interface{})
	Nack()
	Type() string
	Body() string
}

type Event struct {
	MessageId   string `json:"id"`
	ReplyTo     string `json:"reply_to"`
	MessageType string `json:"type"`
	MessageBody string `json:"event"`
	CustomerId  string `json:"customer_id,omitempty"`
	message     *nsq.Message
}

func stringizeMessage(m *nsq.Message) string {
	return fmt.Sprintf(messageFormatString, m.ID, string(m.Body))
}

func validEvent(e *Event) (err error) {
	if e.MessageType == "" {
		err = fmt.Errorf("Message yielded invalid event: %s", stringizeMessage(e.message))
	}
	if e.MessageBody == "" {
		err = fmt.Errorf("Message yielded invalid event: %s", stringizeMessage(e.message))
	}
	return err
}

func NewEvent(msg interface{}) (*Event, error) {
	event := &Event{}

	switch msg := msg.(type) {
	case *nsq.Message:
		event.message = msg
		err := json.Unmarshal(msg.Body, event)
		if err != nil {
			return nil, err
		}
		return event, validEvent(event)
	case EventInterface:
		event.MessageType = msg.Type()
		event.MessageBody = msg.Body()
	default:
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		event.MessageType = reflect.ValueOf(msg).Elem().Type().Name()
		event.MessageBody = string(msgBytes)
	}
	return event, validEvent(event)
}

func (e *Event) Nack() {
	e.message.Requeue(0)
}

func (e *Event) Ack() {
	e.message.Finish()
}

func (e *Event) Reply(reply interface{}) {
	e.message.Finish()
	event, err := NewEvent(reply)
	if err != nil {
		log.Error(err.Error())
		return
	}
	event.ReplyTo = e.MessageId
	eBytes, _ := json.Marshal(event)
	replyProducer.Publish(ReplyTopic, eBytes)
}

func (e *Event) Type() string {
	return e.MessageType
}

func (e *Event) Body() string {
	return e.MessageBody
}
