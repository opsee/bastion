package checker

import (
	"github.com/opsee/bastion/aws"
	"github.com/opsee/bastion/config"
)

type Resolver interface {
	Resolve(*Target) ([]*string, error)
}

// TODO: The resolver should not query the EC2Scanner directly, but
// should go through the instance/group cache instead.
type AWSResolver struct {
	aws aws.EC2Scanner
}

func NewResolver(cfg *config.Config) *AWSResolver {
	resolver := &AWSResolver{
		aws: aws.NewScanner(cfg),
	}
	return resolver
}

func (r *AWSResolver) resolveSecurityGroup(sgid string) ([]*string, error) {
	reservations, err := r.aws.ScanSecurityGroupInstances(sgid)
	if err != nil {
		return nil, err
	}

	targets := make([]*string, len(reservations))
	for i, reservation := range reservations {
		for _, instance := range reservation.Instances {
			// TODO: In some cases this won't be so easy.
			// TODO: Also, god help us if a reservation contains more than one
			// instance
			if instance.PrivateIPAddress != nil {
				targets[i] = instance.PrivateIPAddress
			} else if instance.PublicIPAddress != nil {
				targets[i] = instance.PublicIPAddress
			}
		}
	}
	return targets, nil
}

func (r *AWSResolver) Resolve(target *Target) ([]*string, error) {
	switch target.Type {
	case "sg":
		return r.resolveSecurityGroup(target.Id)
	}

	return []*string{}, nil
}
