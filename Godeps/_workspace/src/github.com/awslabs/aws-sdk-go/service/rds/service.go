package rds

import (
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/aws"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/internal/protocol/query"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/internal/signer/v4"
)

type RDS struct // RDS is a client for Amazon RDS.
{
	*aws.Service
}

// New returns a new RDS client.
func New(config *aws.Config) *RDS {
	if config == nil {
		config = &aws.Config{}
	}

	service := &aws.Service{
		Config:      aws.DefaultConfig.Merge(config),
		ServiceName: "rds",
		APIVersion:  "2014-10-31",
	}
	service.Initialize()

	// Handlers
	service.Handlers.Sign.PushBack(v4.Sign)
	service.Handlers.Build.PushBack(query.Build)
	service.Handlers.Unmarshal.PushBack(query.Unmarshal)
	service.Handlers.UnmarshalMeta.PushBack(query.UnmarshalMeta)
	service.Handlers.UnmarshalError.PushBack(query.UnmarshalError)

	return &RDS{service}
}
