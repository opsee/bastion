package netutil
import (
    "io"
    "sync"
)

type Codec interface {
    ReadMessage(*Message) error
    WriteMessage(*Message) error
    io.Closer
}

type Endpoint struct {
    codec Codec
    client struct {
        mutex sync.Mutex
        seq uint64
        pending map[uint64]interface{}
    }

    server struct {
         registry *Registry
        running sync.WaitGroup

    }
}

