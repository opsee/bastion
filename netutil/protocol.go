package netutil

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
)

type (
	MessageData map[string]interface{}

	MessageId uint64

	Header struct {
		Version uint32    `json:"version"`
		Id      MessageId `json:"id"`
	}

	Message struct {
		*Header
		Data MessageData `json:"message"`
	}

	Request struct {
		*Message
		Command string `json:"command"`
	}

	Reply struct {
		*Message
		InReplyTo MessageId `json:"in_reply_to"`
	}

	Serializable interface {
		Serialize(writer io.Writer) error
	}

	Deserializable interface {
		Deserialize(reader io.Reader) error
	}
)

var (
	crlfSlice        = []byte{'\r', '\n'}
	messageId uint64 = 0
)

func SerializeMessage(writer io.Writer, message interface{}) (err error) {
	if jsonData, err := json.Marshal(message); err != nil {
		log.Error("json.Marshal(): %s", err)
	} else {
		_, err = writer.Write(append(jsonData, crlfSlice...))
	}
	return
}


func DeserializeMessage(reader io.Reader, message interface{}) (err error) {
	bufReader := bufio.NewReader(reader)
	data, isPrefix, err := bufReader.ReadLine()
	if err != nil || isPrefix {
		return err
	} else {
		return json.Unmarshal(data, &message)
	}
}

func NewMessage() *Message {
	return &Message{Header: &Header{Version: 1}, Data: make(MessageData)}
}

func NewRequest(command string) *Request {
	return &Request{Message: &Message{Header: &Header{Version: 1}, Data: make(MessageData)}, Command: command}
}

func NewReply(inReplyTo *ServerRequest) *Reply {
	return &Reply{Message: NewMessage(), InReplyTo: inReplyTo.Id}
}

func (m *Message) Deserialize(reader io.Reader) error {
	return DeserializeMessage(reader, m)
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

func nextMessageId() MessageId {
	return MessageId(atomic.AddUint64(&messageId, 1))
}
