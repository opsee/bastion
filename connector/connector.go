package connector

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"net"
	"reflect"
	"sync"
	"sync/atomic"

	"time"

	"github.com/op/go-logging"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/messaging"
	"github.com/opsee/bastion/netutil"
	"github.com/streamrail/concurrent-map"
)

const (
	protocolVersion = 1
)

type MessageId uint64

var (
	log       = logging.MustGetLogger("connector")
	logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}")
)

type Connected struct {
	Time       int64                `json:"time"`
	CustomerId string               `json:"customer_id"`
	Instance   *config.InstanceMeta `json:"instance"`
}

type Connector struct {
	Conn        net.Conn
	Address     string
	Send        chan<- interface{}
	Recv        <-chan *ConnectorEvent
	config      *config.Config
	metadata    *config.InstanceMeta
	sslConfig   *tls.Config
	reconnCond  *sync.Cond
	sendrcvCond *sync.Cond
	replies     cmap.ConcurrentMap
	counter     uint64
}

type ConnectorEvent struct {
	MessageType string    `json:"type"`
	MessageBody string    `json:"body"`
	Id          MessageId `json:"id"`
	ReplyTo     MessageId `json:"reply_to"`
	Version     uint8     `json:"version"`
	Sent        int64     `json:"sent"`
	connector   *Connector
}

type Status string

func (e *ConnectorEvent) Nack() {
	e.connector.DoReply(e.Id, Status("error"))
}

func (e *ConnectorEvent) Ack() {
	e.connector.DoReply(e.Id, Status("ok"))
}

func (e *ConnectorEvent) Reply(reply interface{}) {
	e.connector.DoReply(e.Id, reply)
}

func (e *ConnectorEvent) Type() string {
	return e.MessageType
}

func (e *ConnectorEvent) Body() string {
	return e.MessageBody
}

func StartConnector(address string, sendbuf int, recvbuf int, metadata *config.InstanceMeta, config *config.Config) *Connector {
	send := make(chan interface{}, sendbuf)
	recv := make(chan *ConnectorEvent, recvbuf)
	connector := &Connector{
		Address:     address,
		Send:        send,
		Recv:        recv,
		config:      config,
		metadata:    metadata,
		sslConfig:   initSSLConfig(config),
		reconnCond:  &sync.Cond{L: &sync.Mutex{}},
		sendrcvCond: &sync.Cond{L: &sync.Mutex{}},
		counter:     0,
	}
	go reconnectLoop(connector)
	go sendLoop(send, connector)
	go recvLoop(recv, connector)
	return connector
}

func (connector *Connector) DoReply(id MessageId, reply interface{}) {
	event, err := connector.makeEvent(reply)
	if err != nil {
		log.Error(err.Error())
		return
	}
	event.ReplyTo = id
	bytes, err := json.Marshal(event)
	if err != nil {
		log.Error(err.Error())
		return
	}
	conn := connector.mustGetConnection()
	_, err = conn.Write(append(bytes, '\r', '\n'))
	if err != nil {
		log.Error(err.Error())
		connector.closeAndSignalReconnect(conn)
	}
}

func initSSLConfig(config *config.Config) *tls.Config {
	if config.CaPath == "" || config.CertPath == "" || config.KeyPath == "" {
		return nil
	}

	cert, err := tls.LoadX509KeyPair(config.CertPath, config.KeyPath)
	if err != nil {
		log.Error("Encountered problem reading SSL config %s", err)
		return nil
	}
	caBytes, err := ioutil.ReadFile(config.CaPath)
	if err != nil {
		log.Error("Encountered a problem reading the CA File %s", err)
		return nil
	}
	caCert, err := x509.ParseCertificate(caBytes)
	if err != nil {
		log.Error("Encountered a problem parsing the CA PEM %s", err)
		return nil
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}
}

func reconnectLoop(connector *Connector) {
	for {
		if connector.Conn == nil {
			connector.sendrcvCond.L.Lock()
			var tryToConnect netutil.BackoffFunction
			tryToConnect = func() (interface{}, error) {
				log.Info("Attempting connection")
				return connectToOpsee(connector)
			}
			retrier := netutil.NewBackoffRetrier(tryToConnect)
			if err := retrier.Run(); err != nil {
				log.Info("error trying to connect")
				continue
			}
			connector.Conn = retrier.Result().(net.Conn)
			connector.sendrcvCond.L.Unlock()
			connector.sendrcvCond.Broadcast()
			log.Info("Successfully connected.")
			connector.sendRegistration()
		}
		connector.reconnCond.L.Lock()
		connector.reconnCond.Wait()
		connector.reconnCond.L.Unlock()
	}
}

func (c *Connector) makeEvent(msg interface{}) (*ConnectorEvent, error) {
	messageType := ""
	messageBody := ""

	switch msg := msg.(type) {
	case messaging.EventInterface:
		messageType = msg.Type()
		messageBody = msg.Body()
	default:
		bytes, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		messageType = reflect.ValueOf(msg).Elem().Type().Name()
		messageBody = string(bytes)
	}

	return &ConnectorEvent{
		MessageType: messageType,
		MessageBody: messageBody,
		Id:          MessageId(atomic.AddUint64(&c.counter, 1)),
		ReplyTo:     MessageId(0),
		Version:     protocolVersion,
		Sent:        time.Now().Unix(),
		connector:   c,
	}, nil
}

func (c *Connector) sendRegistration() {
	connected := &Connected{}
	c.Send <- connected
}

func sendLoop(send <-chan interface{}, connector *Connector) {
	for msg := range send {
		event, err := connector.makeEvent(msg)
		if err != nil {
			log.Error("encountered an error generating the event", err)
			continue
		}

		bytes, err := json.Marshal(event)
		if err != nil {
			log.Error("encountered an error marshalling json", err)
			continue
		}
		conn := connector.mustGetConnection()
		conn.SetWriteDeadline(time.Now().Add(time.Duration(10) * time.Second))
		_, err = conn.Write(append(bytes, '\r', '\n'))
		if err != nil {
			log.Error("encountered an error writing to the connector socket %s", err)
			connector.closeAndSignalReconnect(conn)
		}
	}
}

func recvLoop(recv chan<- *ConnectorEvent, connector *Connector) {
	for {
		conn := connector.mustGetConnection()
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			bytes := scanner.Bytes()
			msg := &ConnectorEvent{}
			err := json.Unmarshal(bytes, msg)
			if err != nil {
				log.Error("encountered an error unmarshalling json: %s", err)
				continue
			}
			msg.connector = connector
			recv <- msg
		}
		log.Error("encountered an error reading from the connector socket %s", scanner.Err())
		connector.closeAndSignalReconnect(conn)
	}
}

func connectToOpsee(connector *Connector) (net.Conn, error) {
	if connector.sslConfig == nil {
		return net.Dial("tcp", connector.Address)
	} else {
		return tls.Dial("tcp", connector.Address, connector.sslConfig)
	}
}

func (connector *Connector) closeAndSignalReconnect(conn net.Conn) {
	connector.sendrcvCond.L.Lock()
	defer connector.sendrcvCond.L.Unlock()
	if conn != connector.Conn {
		return
	}
	conn.Close()
	connector.Conn = nil
	connector.reconnCond.Signal()
}

func (connector *Connector) mustGetConnection() net.Conn {
	connector.sendrcvCond.L.Lock()
	conn := connector.Conn
	for conn == nil {
		connector.sendrcvCond.Wait()
		conn = connector.Conn
	}
	connector.sendrcvCond.L.Unlock()
	return conn
}
