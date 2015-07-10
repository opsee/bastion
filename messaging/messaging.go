/* Messaging acts as a gateway between the Bastion and whatever is used
 * to communicate between Bastion components. It's responsibility is to provide
 * convenient interfaces to facilitate safe, structured message passing to
 * prevent lost/dead messages. All "wire-level" serialization/deserialization
 * should occur within the messaging package so that plugging different
 * messaging subsystems in (or ripping them out entirely) is easier.
 */
package messaging

import (
	"os"

	"github.com/opsee/bastion/logging"
)

// TODO: greg: Refactor Consumer/Producer into interfaces.

var (
	logger = logging.GetLogger("messaging")
)

func getNsqdURL() string {
	nsqdURL := os.Getenv("NSQD_HOST")
	return nsqdURL
}

const (
	ProtocolVersion = 1
)

type MessageId uint64

type Message struct {
	// Routing, flow control
	Id         MessageId `json:"id"`
	Version    uint8     `json:"version"`
	Sent       int64     `json:"sent"`
	CustomerId string    `json:"customer_id"`
	InstanceId string    `json:"instance_id"`
	Ttl        uint32    `json:"ttl"` // in ms
	// Application layer
	Command    string                 `json:"command"`
	Attributes map[string]interface{} `json:"attributes"`
	Body       Event                  `json:"body"`
}

type EventInterface interface {
	Ack()
	Nack()
	Type() string
	Body() string
}

type Event struct {
	MessageType string `json:"type"`
	MessageBody string `json:"event"`
	message     *nsq.Message
}

func NewEvent(msg *nsq.Message) (EventInterface, error) {
	e := &Event{message: msg}
	err := json.Unmarshal(msg.Body, e)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	return e, nil
}

func (e *Event) Nack() {
	e.message.Requeue(0)
}

func (e *Event) Ack() {
	e.message.Finish()
}

func (e *Event) Type() string {
	return e.MessageType
}

func (e *Event) Body() string {
	return e.MessageBody
}
