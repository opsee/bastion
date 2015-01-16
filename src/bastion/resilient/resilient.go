package resilient

import (
		"errors"
		"fmt"
		"time"
		"crypto/tls"
		"crypto/x509"
		"encoding/json"
		"io/ioutil"
		"encoding/binary"
		bioutil "bastion/ioutil"
)

type ResilientConn struct {
	closed			bool
	address			string
	sendChan		chan interface{}
	config			*tls.Config
	conn 			*tls.Conn
	recvChan		chan interface{}
	connected		bool
	connChan		chan bool
}

func loadConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	caPool := x509.NewCertPool()
	data, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	if !caPool.AppendCertsFromPEM(data) {
		return nil, errors.New("capool append PEM failed")
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config {
		Certificates: []tls.Certificate{cert},
		RootCAs: caPool}, nil
}

func Start(address, caFile, certFile, keyFile string) (*ResilientConn, error) {
	config, err := loadConfig(caFile, certFile, keyFile)
	if err != nil {
		return nil, err
	}
	conn := &ResilientConn {
		address : address,
		sendChan : make(chan interface{}, 10000),
		config : config,
		recvChan : make(chan interface{}, 10000),
		connected : false,
		connChan : make(chan bool, 1)}
	go conn.loop(10)
	return conn, nil
}

func (rc *ResilientConn) Close() {
	rc.closed = true
	close(rc.sendChan)
	close(rc.recvChan)
	close(rc.connChan)
	if rc.conn != nil {
		rc.conn.Close()
	}
}

func (rc *ResilientConn) Send(v interface{}) {
	rc.sendChan <- v
}

func (rc *ResilientConn) Recv() interface{} {
	return <- rc.recvChan
}

func (rc *ResilientConn) loop(timeout int) {
	for {
		if rc.closed {
			return
		}
		timer := rc.reconnLoop(timeout, &timeout)
		if timer != nil {
			<- timer
			timeout *= 2
		}
	}
}

func (rc *ResilientConn) reconnLoop(startingTimeout int, timeout *int) <-chan time.Time {
	if rc.conn == nil {
		conn, err := tls.Dial("tcp", rc.address, rc.config)
		if err != nil {
			fmt.Println("encountered error trying to connect to opsee backend:", err)
			return time.After(time.Duration(*timeout) * time.Second)
		}
		rc.conn = conn
		*timeout = startingTimeout
		rc.connected = true
		rc.connChan <- true
		go rc.sender()
	}
	recvBuffer, err := bioutil.ReadFramed(rc.conn)
	if err != nil {
		rc.conn.Close()
		rc.conn = nil
		rc.connected = false
	}
	if len(recvBuffer) == 0 {
		return nil
	}

	var v interface{}
	err = json.Unmarshal(recvBuffer, &v)
	if err != nil {
		fmt.Println("error unmarshalling json:", err)
	} else {
		rc.recvChan <- v
	}
	return nil
}

func (rc *ResilientConn) sender() {
	for {
		toSend := <- rc.sendChan
		buff, err := json.Marshal(&toSend)
		if err != nil {
			fmt.Println("encountered error marshalling object:", err)
			continue
		}
		size := uint16(len(buff))
		if rc.conn == nil {
			return
		}
		err = binary.Write(rc.conn, binary.BigEndian, &size)
		n, err := rc.conn.Write(buff)
		if err != nil || uint16(n) != size {
			fmt.Println("encountered error writing data to conn:", err)
			return
		}
	}
}

func (rc *ResilientConn) IsConnected() bool {
	return rc.connected
}

func (rc *ResilientConn) WaitConnect() bool {
	return <- rc.connChan
}