package netutil

import (
	"bytes"
	"encoding/binary"
	"sync"
	//    "errors"
	//    "fmt"a
	"io"
)

type Pipeline struct {
	client *Client
	buf    *bytes.Buffer
	count  int32
	sync.Mutex
}

func NewPipeline(c *Client) (p *Pipeline) {
	p = &Pipeline{client: c}
	p.buf = bytes.NewBuffer(nil)
	return p
}

// Send a request over the Pipeline. This collects the request data in an internal
// buffer before flushing to the actual connection. This means the request wont actual
// be delivered until Flush() is called
func (p *Pipeline) Send(req []byte) error {
	p.Lock()
	defer p.Unlock()
	p.count++
	_, err := writeDataWithLength(req, p.buf)
	return err
}

// Flush actually delivers all the buffered request data to the connection. It then
// blocks waiting for all the responses from the server. These requests are returned
//// in order and stored in an slice and returned as responses
//func (p *Pipeline) Flush() (responses [][]byte, err error) {
//    conn, err := p.client.pool.Take()
//    if err != nil {
//        return nil, err
//    }
//    // Write the initial byte as -the count of the messages
//    err = binary.Write(conn, binary.BigEndian, int32(-p.count))
//    if err != nil {
//        return nil, err
//    }
//    // Flush the whole buffer
//    conn.Write(p.buf.Bytes())
//    var responseCount int32
//    err = binary.Read(conn, binary.BigEndian, &responseCount)
//    if err != nil {
//        return nil, err
//    }
//    if -responseCount != p.count {
//        return nil, errors.New(fmt.Sprintf("Mismatched number of responses for pipeline request. Expected %d, got %d", p.count, -responseCount))
//    }
//    responses = make([][]byte, -responseCount)
//    for i := int32(0); i < -responseCount; i++ {
//        responses[i], err = readDataWithLength(conn)
//    }
//    p.client.pool.Return(conn)
//    return
//}

func writeDataWithLength(data []byte, buf io.Writer) (length int, err error) {
	err = binary.Write(buf, binary.BigEndian, int32(len(data)))
	if err != nil {
		return 0, err
	}
	n, err := buf.Write(data)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func readDataWithLength(conn io.Reader) (data []byte, err error) {
	var size int32
	err = binary.Read(conn, binary.BigEndian, &size)
	if err != nil {
		return nil, err
	}
	data = make([]byte, size)
	_, err = io.ReadFull(conn, data)
	if err != nil {
		return nil, err
	}
	return
}
