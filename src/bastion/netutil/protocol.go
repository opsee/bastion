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

func NewMessage() *Message {
	header := &Header{Version: 1, Id: nextMessageId()}
	return &Message{Header: header, Data: make(MessageData)}
}

func (h *Header) String() string {
	return fmt.Sprintf("Header@%p[id=%d version=%d]", h, h.Id, h.Version)
}

type Request struct {
	*Message
	Command string `json:"command"`
}

func NewRequest(command string) *Request {
	return &Request{NewMessage(), command}
}

func (r *Request) String() string {
	return fmt.Sprintf("Message@%p[command=%s id=%d version=%d messagedata=%v]", r, r.Command, r.Id, r.Version, r.Data)
}

type Reply struct {
	*Message
	InReplyTo MessageId `json:"in_reply_to"`
}

func NewReply(inReplyTo *Request) *Reply {
	return &Reply{Message: NewMessage(), InReplyTo: inReplyTo.Id}
}

func (r *Reply) String() {}

var requestId uint64 = 0

func nextMessageId() MessageId {
	return MessageId(atomic.AddUint64(&requestId, 1))
}
