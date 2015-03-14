package netutil

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
)

const (
	MsgKeepAlive string = "keepalive"
)

var (
	crlfSlice = []byte{'\r', '\n'}
)

type MessageData map[string]interface{}
type MessageId uint64

type Header struct {
	Version uint32    `json:"version"`
	Id      MessageId `json:"id`
}

type Message struct {
	*Header
	Data MessageData `json:"message"`
}

type Request struct {
	*Message
	Command string `json:"command"`
}

type Reply struct {
	*Message
	InReplyTo MessageId `json:"in_reply_to"`
}

type Serializable interface {
	Serialize(writer io.Writer) error
}

type Deserializable interface {
	Deserialize(reader io.Reader) error
}

func SerializeMessage(writer io.Writer, message interface{}) error {
	if jsonData, err := json.Marshal(message); err == nil {
		if _, err = writer.Write(append(jsonData, crlfSlice...)); err != nil {
			return err
		}
	}
	return nil
}

func DeserializeMessage(reader io.Reader, message interface{}) error {
	bufReader := bufio.NewReader(reader)
	data, isPrefix, err := bufReader.ReadLine()
	if isPrefix || err != nil {
		if isPrefix {
			log.Panic("[PANIC]: Message.Deserialize: partial read shouldn't happenen: ", err)
		}
		return err
	} else {
		return json.Unmarshal(data, &message)
	}
}

func NewMessage() *Message {
	return &Message{Header: &Header{Version: 1}, Data: make(MessageData)}
}

func NewRequest(command string, incrementId bool) *Request {
	request := &Request{Message: &Message{Header: &Header{Version: 1}, Data: make(MessageData)}, Command: command}
	if incrementId {
		request.Id = nextMessageId()
	}
	return request
}

func NewReply(inReplyTo *Request, incrementId bool) *Reply {
	reply := &Reply{Message: NewMessage(), InReplyTo: inReplyTo.Id}
	if incrementId {
		reply.Id = nextMessageId()
	}
	return reply
}

func (h *Header) String() string {
	return fmt.Sprintf("Header@%p[id=%d version=%d]", h, h.Id, h.Version)
}

func (r *Request) String() string {
	return fmt.Sprintf("Request@%p[command=%s id=%d version=%d messagedata=%v]", r, r.Command, r.Id, r.Version, r.Data)
}

func (r *Reply) String() string {
	return fmt.Sprintf("Reply@%p[id=%d version=%d in_reply_to=%d messagedata=%v]", r, r.Id, r.Version, r.InReplyTo, r.Data)

}

var requestId uint64 = 0

func nextMessageId() MessageId {
	return MessageId(atomic.AddUint64(&requestId, 1))
}
