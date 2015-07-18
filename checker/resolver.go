package checker

import (
	"github.com/opsee/bastion/aws"
	"github.com/opsee/bastion/config"
)

type Resolver struct {
	aws aws.EC2Scanner
}

func NewResolver(cfg *config.Config) *Resolver {
	resolver := &Resolver{
		aws: aws.NewScanner(cfg),
	}
	return resolver
}

func (r *Resolver) resolveSecurityGroup(sgid string) ([]Target, error) {
	return nil, nil
}

func (r *Resolver) Resolve(target Target) ([]Target, error) {
	switch target.Type {
	case "sg":
		return r.resolveSecurityGroup(target.Id)
	}

	return []Target{}, nil
}
