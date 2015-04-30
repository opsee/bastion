package netutil

import (
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/op/go-logging"
	"net"
)

var (
	log       = logging.MustGetLogger("netutil")
	logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}")
)

func init() {
	logging.SetLevel(logging.DEBUG, "bastion.json-tcp")
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

func MustGetHostname() (hostname string) {
	hostname = "localhost"
	if ifaces, err := net.InterfaceAddrs(); err != nil {
		log.Panicf("InterfaceAddrs(): %s", err)
	} else {
		for _, iface := range ifaces {
			if ifaceip, _, err := net.ParseCIDR(iface.String()); err != nil {
				log.Error("ParseCIDR: %s", err)
				continue
			} else {
				log.Info("Iface: %s, IfaceIP: %s", iface.String(), ifaceip.String())
				if ipaddrs, err := net.LookupAddr(ifaceip.String()); err != nil {
					log.Error("LookupAddr(): %s", err)
					continue
				} else {
					for _, name := range ipaddrs {
						if !ifaceip.IsLoopback() {
							hostname = name
						}
					}
				}
			}
		}
	}
	return
}
