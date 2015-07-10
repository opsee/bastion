package connector

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/op/go-logging"
	"github.com/opsee/bastion/aws"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/messaging"
	"github.com/opsee/bastion/netutil"
)

var (
	log       = logging.MustGetLogger("connector")
	logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}")
)

type Connector struct {
	Conn         net.Conn
	Address      string
	Send         chan *netutil.Message
	Recv         chan *netutil.Message
	config       *config.Config
	metadata     *aws.InstanceMeta
	sslConfig    *tls.Config
	reconnCond   *sync.Cond
	sendrcvCond  *sync.Cond
	messageMaker *messaging.MessageMaker
}

func StartConnector(address string, sendbuf int, recvbuf int, metadata *aws.InstanceMeta, config *config.Config) *Connector {
	connector := &Connector{
		Address:      address,
		Send:         make(chan *netutil.Message, sendbuf),
		Recv:         make(chan *netutil.Message, recvbuf),
		config:       config,
		metadata:     metadata,
		sslConfig:    initSSLConfig(config),
		reconnCond:   &sync.Cond{L: &sync.Mutex{}},
		sendrcvCond:  &sync.Cond{L: &sync.Mutex{}},
		messageMaker: *messaging.MessageMaker,
	}
	go reconnectLoop(connector)
	go sendLoop(connector)
	go recvLoop(connector)
	return connector
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
			sendRegistration(connector)
		}
		connector.reconnCond.L.Lock()
		connector.reconnCond.Wait()
		connector.reconnCond.L.Unlock()
	}
}

func sendRegistration(connector *Connector) {
	msg := connector.MakeMessage("connected", nil)
	msg.Attributes["state"] = "connected"
	connector.Send <- msg
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
			msg := &netutil.Message{}
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
