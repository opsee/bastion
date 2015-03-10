package netutil

import (
	"fmt"
	"sync/atomic"
)

type Message map[string]interface{}
type RequestId uint64
type RequestHandler func(*Request, Connection)

type Header struct {
	Version uint      `json:"version"`
	Id      RequestId `json:"id"`
}

func (h *Header) String() string {
	return fmt.Sprintf("Header@%p[id=%d version=%d]", h, h.Id, h.Version)
}

type Request struct {
	*Header
	Command string  `json:"command"`
	Message Message `json:"message"`
}

func (r *Request) String() string {
	return fmt.Sprintf("Message@%p[command=%s id=%d version=%d message=%v]", r, r.Command, r.Id, r.Version, r.Message)
}

type Reply struct {
	*Header
	RequestId RequestId `json:"request_id"`
	Message   Message   `json:"message"`
}

func (r *Reply) String() {}

func init() {
	requestId = 0
}

var requestId uint64

func nextRequestId() uint64 {
	return atomic.AddUint64(&requestId, 1)
}
