package netutil

import (
	"errors"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/op/go-logging"
	"net"
	"os"
)

var (
	log       = logging.MustGetLogger("netutil")
	logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}")
)

func init() {
	logging.SetLevel(logging.DEBUG, "netutil")
	logging.SetFormatter(logFormat)
}

func ConnectTCP(address string, c Client) (client *BaseClient, err error) {
	if tcpAddr, err := net.ResolveTCPAddr("tcp", address); err == nil {
		if tcpConn, err := net.DialTCP("tcp", nil, tcpAddr); err == nil {
			client = &BaseClient{TCPConn: tcpConn, Address: address, callbacks: c}
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
	if oshostname, err := os.Hostname(); err == nil {
		hostname = oshostname
	}
	if localIP, err := getLocalIP(); err == nil {
		if hostnames, err := net.LookupAddr(localIP.String()); err == nil {
			hostname = hostnames[0]
		}
	}
	log.Info("hostname: %s", hostname)
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
