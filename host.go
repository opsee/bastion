package bastion
import "net"


type BastionHost struct {
	InstanceId string	`json:"instance_id"`
	OsHostname string	`json:"os_hostname"`
	LocalIpAddr net.IPAddr	`json:"host_ip"`
	DnsHostname string	`json:"dns_hostname"`
}

func ThisBastion() {
	b := &BastionHost{}
	return b
}