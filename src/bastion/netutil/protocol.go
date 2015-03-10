package netutil

import (
	"fmt"
	"sync/atomic"
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

func NewMessage() *Message {
	header := &Header{Version: 1}
	return &Message{Header: header, Data: make(MessageData)}
}

func NewRequest(command string) *Request {
	return &Request{Message: &Message{Header: &Header{Version: 1}, Data: make(MessageData)}, Command: command}
}

func NewReply(inReplyTo *Request) *Reply {
	reply := &Reply{Message: NewMessage(), InReplyTo: inReplyTo.Id}
	reply.Id = nextMessageId()
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
