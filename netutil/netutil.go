package netutil

import (
	"errors"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/op/go-logging"
	"net"
	"os"
	"time"
)

var (
	log       = logging.MustGetLogger("netutil")
	logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}")
)

const (
	protocolVersion   = 1
	minDefaultTtl     = 10
	noInstanceId      = "i-0000000"
	unknownHostname   = "unknown-hostname"
	unknownCustomerId = "unknown-customer"
)

func init() {
	logging.SetLevel(logging.DEBUG, "netutil")
	logging.SetFormatter(logFormat)
}

type Message struct {
	Id         uint64                 `json:"id"`
	Version    uint8                  `json:"version"`
	Command    string                 `json:"command"`
	Sent       time.Time              `json:"sent"`
	Attributes map[string]interface{} `json:"attributes"`
	Host       string                 `json:"host"` // Defaults to os.Hostname()
	CustomerId string                 `json:"customer_id"`
	InstanceId string                 `json:"instance_id"`
}

type Event struct {
	Ttl         float32     `json:"ttl"`
	Tags        []string    `json:"tags"`
	State       string      `json:"state"`
	Service     string      `json:"service"`
	Metric      interface{} `json:"metric"` // Could be Int, Float32, Float64
	Description string      `json:"description"`
}

type EventMessage struct {
	Message
	Event
}

type MessageMaker interface {
	NewEventMessage() *EventMessage
}

type EventMessageMaker struct {
	Ttl        float32
	InstanceId string
	Hostname   string
}

func NewEventMessageMaker(defaultTtl float32, defaultInstanceId string, defaultHostname string) *EventMessageMaker {
	log.Info("ttl: %v iid: %s hostname: %s", defaultTtl, defaultInstanceId, defaultHostname)
	if defaultTtl < 1.0 {
		defaultTtl = minDefaultTtl
	}
	if defaultInstanceId == "" {
		defaultInstanceId = noInstanceId
	}
	if defaultHostname == "" {
		var err error
		if defaultHostname, err = os.Hostname(); err != nil {
			defaultHostname = unknownHostname
		}
	}
	return &EventMessageMaker{Ttl: defaultTtl, InstanceId: defaultInstanceId, Hostname: defaultHostname}
}

func (e *EventMessageMaker) NewEventMessage() *EventMessage {
	m := &EventMessage{}
	m.Id = uint64(nextMessageId())
	m.Version = protocolVersion
	m.Command = "default"
	m.Sent = time.Now()
	m.Attributes = make(map[string]interface{})
	m.Host = string([]byte(e.Hostname))
	m.InstanceId = string([]byte(e.InstanceId))
	m.Ttl = e.Ttl
	m.Service = "default"
	m.CustomerId = unknownCustomerId
	return m
}

func ConnectTCP(address string, c Client) (client *BaseClient, err error) {
	if tcpAddr, err := net.ResolveTCPAddr("tcp", address); err == nil {
		if tcpConn, err := net.DialTCP("tcp", nil, tcpAddr); err == nil {
			client = &BaseClient{Conn: tcpConn, Address: address, callbacks: c}
			return client, nil
		} else {
			return client, err
		}
	}
	return
}

func ListenTCP(address string, s ServerCallbacks) (server TCPServer, err error) {
	server = struct {
		*BaseServer
		ServerCallbacks
	}{NewServer(address, s), s}
	err = server.Serve()
	return
}

func GetHostname() (hostname string, err error) {
	log.Info("GetHostname()")
	if oshostname, err := os.Hostname(); err == nil {
		hostname = oshostname
	} else {
		log.Error("os.Hostname(): %s", err)
	}
	if localIP, err := getLocalIP(); err == nil {
		if hostnames, err := net.LookupAddr(localIP.String()); err == nil {
			hostname = hostnames[0]
		} else {
			log.Error("LookupAddr(): %s", err)
		}
	} else {
		log.Error("getLocalIP: %s", err)
	}
	return
}

func GetHostnameDefault(defaultHostname string) (hostname string) {
	if hostname, err := GetHostname(); err != nil {
		return defaultHostname
	} else {
		return hostname
	}
}

func getLocalIP() (net.IP, error) {
	tt, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, t := range tt {
		aa, err := t.Addrs()
		if err != nil {
			return nil, err
		}
		for _, a := range aa {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			v4 := ipnet.IP.To4()
			if v4 == nil || v4[0] == 127 { // loopback address
				continue
			}
			return v4, nil
		}
	}
	return nil, errors.New("cannot find local IP address")
}
