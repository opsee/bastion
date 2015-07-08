package netutil

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opsee/bastion/logging"
)

var (
	logger = logging.GetLogger("netutil")
)

const (
	protocolVersion   = 1
	minDefaultTtl     = 100 // in ms
	noInstanceId      = "i-0000000"
	unknownHostname   = "unknown-hostname"
	unknownCustomerId = "unknown-customer"
)

type HostInfo struct {
	CustomerId string `json:"customer_id"`
	RegionId   string `json:"region_id"`
	ZoneId     string `json:"zone_id"`
	InstanceId string `json:"instance_id"`
	Hostname   string `json:"hostname"`
	IpAddr     string `json:"ip_address"`
}

var _init_ctx sync.Once
var _hostInfo *HostInfo

func GetHostInfo() *HostInfo {
	if _hostInfo == nil {
		panic("GetHostInfo() called before initialization.")
	}
	return _hostInfo
}

func InitHostInfo(cid string, rid string, zid string, iid string, hostname string, ipaddr string) {
	_init_ctx.Do(
		func() {
			_hostInfo = &HostInfo{CustomerId: cid,
				RegionId:   rid,
				ZoneId:     zid,
				InstanceId: iid,
				Hostname:   hostname,
				IpAddr:     ipaddr}
		})
}

type MessageId uint64

type Message struct {
	// Routing, flow control
	Id         MessageId `json:"id"`
	Version    uint8     `json:"version"`
	Sent       int64     `json:"sent"`
	CustomerId string    `json:"customer_id"`
	InstanceId string    `json:"instance_id"`
	Ttl        uint32    `json:"ttl"` // in ms
	// Application layer
	Command    string                 `json:"command"`
	Attributes map[string]interface{} `json:"attributes"`
	Body       MessageBody            `json:"body"`
}

type MessageBody struct {
	Type string `json:"type"`
	Body string `json:"body"`
}

type MessageMaker struct {
	Ttl        uint32
	InstanceId string
	Hostname   string
	CustomerId string
}

func NewMessageMaker(defaultTtl uint32, defaultInstanceId string, defaultHostname string, defaultCustomerId string) *MessageMaker {
	logger.Info("ttl: %v iid: %s hostname: %s customerId: %s", defaultTtl, defaultInstanceId, defaultHostname, defaultCustomerId)
	if defaultTtl < 1 {
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
	if defaultCustomerId == "" {
		defaultCustomerId = unknownCustomerId
	}
	return &MessageMaker{Ttl: defaultTtl, InstanceId: defaultInstanceId, Hostname: defaultHostname, CustomerId: defaultCustomerId}
}

func (e *MessageMaker) NewMessage() *Message {
	m := &Message{}
	m.Id = nextMessageId()
	m.Version = protocolVersion
	m.Sent = time.Now().UTC().Unix()
	m.InstanceId = string([]byte(e.InstanceId))
	m.Ttl = e.Ttl
	m.CustomerId = e.CustomerId
	m.Attributes = make(map[string]interface{})
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
	logger.Info("GetHostname()")
	if oshostname, err := os.Hostname(); err == nil {
		hostname = oshostname
	} else {
		logger.Error("os.Hostname(): %s", err)
	}
	if localIP, err := getLocalIP(); err == nil {
		if hostnames, err := net.LookupAddr(localIP.String()); err == nil {
			hostname = hostnames[0]
		} else {
			logger.Error("LookupAddr(): %s", err)
		}
	} else {
		logger.Error("getLocalIP: %s", err)
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

func nextMessageId() MessageId {
	return MessageId(atomic.AddUint64(&messageId, 1))
}

var (
	crlfSlice        = []byte{'\r', '\n'}
	messageId uint64 = 0
)

func SerializeMessage(writer io.Writer, message interface{}) (err error) {
	if jsonData, err := json.Marshal(message); err != nil {
		logger.Error("json.Marshal(): %s", err)
	} else {
		_, err = writer.Write(append(jsonData, crlfSlice...))
	}
	return
}

func DeserializeMessage(reader io.Reader, message interface{}) (err error) {
	bufReader := bufio.NewReader(reader)
	data, isPrefix, err := bufReader.ReadLine()
	if err != nil || isPrefix {
		return err
	} else {
		return json.Unmarshal(data, &message)
	}
}
