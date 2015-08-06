package netutil

import (
	"errors"

	"net"
	"os"

	"github.com/opsee/bastion/logging"
)

var (
	logger = logging.GetLogger("netutil")
)

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
