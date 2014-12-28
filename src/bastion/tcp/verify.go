package tcp

import (
		"net"
		"fmt"
)

type TcpVerify struct {
	host string
	port int
	cchan chan *Conn
	echan chan Err
}

func New(host string, port int) *TcpVerify {
	ver := &TcpVerify {
		host: 		host
		port:		port
		cchan:		make(chan *Conn, 1)
		echan:		make(chan Err, 1)
	}
	go ver.start()
	return &ver
}

func (v *TcpVerify) start() {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", v.host, v.port))
	if err != nil {
		v.echan <- err
		return
	}
	v.cchan <- conn
	return
}

func (v *TcpVerify) IsDone() bool {
	select {
	case conn <- cchan:
		cchan <- conn
		return true
	case err <- echan:
		echan <- err
		return true
	default:
		return false
	}
}

func (v *TcpVerify) IsErr() bool {
	select {
	case conn <- cchan:
		cchan <- conn
		return false
	case err <- echan:
		echan <- err
		return true
	default:
		return false
	}
}

func (v *TcpVerify) IsSuccess() bool {
	select {
	case conn <- cchan:
		cchan <- conn
		return true
	case err <- echan:
		echan <- err
		return false
	default:
		return false
	}
}

func (v *TcpVerify) Err() Err {
	select {
	case err <- echan:
		echan <- err
		return err
	default:
		return nil
	}
}

func (v *TcpVerify) Conn() Conn {
	select {
	case conn <- cchan:
		cchan <- conn
		return conn
	default:
		return nil
	}
}