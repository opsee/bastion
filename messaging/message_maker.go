package messaging

import (
	"github.com/opsee/bastion/aws"
	"github.com/opsee/bastion/config"
)

type MessageMaker struct {
	counter  uint64
	ttl      uint32
	config   *config.Config
	metadata *aws.InstanceMeta
}

func NewMessageMaker(defaultTtl uint32, config *config.Config, metadata *aws.InstanceMeta) *MessageMaker {
	return &MessageMaker{
		counter:  0,
		Ttl:      defaultTtl,
		config:   config,
		metadata: metadata,
	}
}

func (mm *MessageMaker) MakeMessage() Message {
	m := &netutil.EventMessage{}
	m.Id = netutil.MessageId(atomic.AddUint64(&mm.counter, 1))
	m.Version = protocolVersion
	m.Sent = time.Now().Unix()
	m.CustomerId = c.config.CustomerId
	m.InstanceId = c.metadata.InstanceId
	m.Host = c.metadata.Hostname
	m.Attributes = make(map[string]interface{})
	m.Time = time.Now().Unix()
	m.Metric = 0.0
	m.Ttl = mm.Ttl
	return m
}
