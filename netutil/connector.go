package netutil

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"net"
	"sync"
	"time"
)

type SSLOptions struct {
	CertFile string
	KeyFile  string
	CaFile   string
}

type Connector struct {
	Conn        net.Conn
	Address     string
	Send        chan *EventMessage
	Recv        chan *EventMessage
	sslConfig   *tls.Config
	reconnCond  *sync.Cond
	sendrcvCond *sync.Cond
}

func StartConnector(address string, sendbuf int, recvbuf int, options *SSLOptions) *Connector {
	connector := &Connector{
		Address:     address,
		Send:        make(chan *EventMessage, sendbuf),
		Recv:        make(chan *EventMessage, recvbuf),
		sslConfig:   initSSLConfig(options),
		reconnCond:  &sync.Cond{L: &sync.Mutex{}},
		sendrcvCond: &sync.Cond{L: &sync.Mutex{}},
	}
	go reconnectLoop(connector)
	go sendLoop(connector)
	go recvLoop(connector)
	return connector
}

func initSSLConfig(options *SSLOptions) *tls.Config {
	if options == nil {
		return nil
	}

	cert, err := tls.LoadX509KeyPair(options.CertFile, options.KeyFile)
	if err != nil {
		log.Error("Encountered problem reading SSL config %s", err)
		return nil
	}
	caBytes, err := ioutil.ReadFile(options.CaFile)
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
			var tryToConnect BackoffFunction
			tryToConnect = func() (interface{}, error) {
				return connectToOpsee(connector)
			}
			connector.Conn = NewBackoffRetrier(tryToConnect).Result().(net.Conn)
			connector.sendrcvCond.L.Unlock()
			connector.sendrcvCond.Broadcast()
		}
		connector.reconnCond.Wait()
	}
}

func sendLoop(connector *Connector) {
	for msg := range connector.Send {
		bytes, err := json.Marshal(msg)
		if err != nil {
			log.Error("encountered an error marshalling json: %s", err)
			continue
		}
		conn := mustGetConnection(connector)
		conn.SetWriteDeadline(time.Now().Add(time.Duration(10) * time.Second))
		_, err = conn.Write(append(bytes, '\r', '\n'))
		if err != nil {
			log.Error("encountered an error writing to the connector socket %s", err)
			closeAndSignalReconnect(conn, connector)
		}
	}
}

func recvLoop(connector *Connector) {
	for {
		conn := mustGetConnection(connector)
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			bytes := scanner.Bytes()
			msg := &EventMessage{}
			err := json.Unmarshal(bytes, msg)
			if err != nil {
				log.Error("encountered an error unmarshalling json: %s", err)
				continue
			}
			connector.Recv <- msg
		}
		log.Error("encountered an error reading from the connector socket %s", scanner.Err())
		closeAndSignalReconnect(conn, connector)
	}
}

func connectToOpsee(connector *Connector) (net.Conn, error) {
	if connector.sslConfig == nil {
		return net.Dial("tcp", connector.Address)
	} else {
		return tls.Dial("tcp", connector.Address, connector.sslConfig)
	}
}

func closeAndSignalReconnect(conn net.Conn, connector *Connector) {
	connector.sendrcvCond.L.Lock()
	defer connector.sendrcvCond.L.Unlock()
	if conn != connector.Conn {
		return
	}
	conn.Close()
	connector.Conn = nil
	connector.reconnCond.Signal()
}

func mustGetConnection(connector *Connector) net.Conn {
	connector.sendrcvCond.L.Lock()
	conn := connector.Conn
	for conn == nil {
		connector.sendrcvCond.Wait()
		conn = connector.Conn
	}
	connector.sendrcvCond.L.Unlock()
	return conn
}
