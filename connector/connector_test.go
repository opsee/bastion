package connector

import (
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/testing/net"
	// "github.com/stretchr/testify/assert"
	// "github.com/stretchr/testify/mock"
	"testing"
)

var instanceMeta = config.InstanceMeta{
	InstanceId:       "i-88888",
	Architecture:     "",
	ImageId:          "",
	InstanceType:     "m3.medium",
	Hostname:         "localhost",
	KernelId:         "",
	RamdiskId:        "",
	Region:           "us-west-1",
	Version:          "",
	PrivateIp:        "",
	AvailabilityZone: "us-west-1c",
}

var nonSSLConfig = config.Config{
	CaPath:   "",
	CertPath: "",
	KeyPath:  "",
}

func TestConnectorTCP(t *testing.T) {
	server := net.NetServer(30000, t)

	// connector := 
	StartConnector("localhost:30000", 100, 100, &instanceMeta, &nonSSLConfig)
	// serverConn := 
	<-server.Startup

}
