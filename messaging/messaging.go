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
	"os"

	"github.com/bitly/go-nsq"
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
