package netutil

import (
    "bufio"
    "fmt"
    "net"
    "github.com/opsee/bastion/util"
)

type (
	SslOptions map[string]string

	ServerCallbacks interface {
		RequestReceived(*Connection, interface{}) (reply interface{}, keepGoing bool)
		ConnectionMade(*Connection) (keepGoing bool)
		ConnectionLost(*Connection, error)
		SslOptions() SslOptions
	}

	Listener interface {
		net.Listener
		Serve() error
		Stop()
		Join()
		Listen() (net.Listener, error)
	}



	BaseClient struct {
		net.Conn
		Address   string
//    callbacks Client
	}
)

func (c *BaseClient) SendEvent(event *EventMessage) error {
    log.Info("sendEvent: %+v", event)
    return SerializeMessage(c, event)
}

type Connection struct {
    net.Conn
    *bufio.Reader
    id         int64
    span       *util.Span
    requestNum util.AtomicCounter
}

func NewConnection(conn net.Conn) *Connection {
    var connectionId = nextConnectionId.Increment()
    return &Connection{Conn: conn,
        Reader: bufio.NewReader(conn),
        span:   util.NewSpan(fmt.Sprintf("conn-%d-%s-%s", connectionId, conn.RemoteAddr(), conn.LocalAddr())),
        id:     connectionId,
    }
}

func (c *Connection) loop() (err error) {
    for {
        c.requestNum.Increment()
        //		select {
        //		case <- serverCtx.Done():
        //			break
        //		}
    }
    c.span.CollectMemStats()
    log.Debug(c.span.JSON())
    return nil
}

func (c *Connection) readRequest() (message *EventMessage, err error) {
    return message, DeserializeMessage(c.Conn, message)
}

func (c *Connection) handleRequest(request *EventMessage) (err error) {
    return nil
}

func (c *Connection) Close() error {
//    atomic.StoreInt32(&c.server.exit, 1)
    return c.Conn.Close()
}

var nextConnectionId util.AtomicCounter
