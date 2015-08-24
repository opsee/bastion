package checker

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/scanner"
)

type Resolver interface {
	Resolve(*Target) ([]*string, error)
}

// TODO: The resolver should not query the EC2Scanner directly, but
// should go through the instance/group cache instead.
type AWSResolver struct {
	sc scanner.EC2Scanner
}

func NewResolver(cfg *config.Config) Resolver {
	resolver := &AWSResolver{
		sc: scanner.NewScanner(cfg),
	}
	return resolver
}

// TODO: In some cases this won't be so easy.
// TODO: Also, god help us if a reservation contains more than one
// instance
func getAddrFromInstance(instance *ec2.Instance) *string {
	var addr *string
	if instance.PrivateIPAddress != nil {
		addr = instance.PrivateIPAddress
	} else if instance.PublicIPAddress != nil {
		addr = instance.PublicIPAddress
	}

	return addr
}

func (r *AWSResolver) resolveSecurityGroup(sgid string) ([]*string, error) {
	reservations, err := r.sc.ScanSecurityGroupInstances(sgid)
	if err != nil {
		return nil, err
	}

	logger.Debug("reservations: %v", reservations)

	targets := make([]*string, 0)
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			targets = append(targets, getAddrFromInstance(instance))
		}
	}
	return targets, nil
}

func (r *AWSResolver) resolveELB(elbId string) ([]*string, error) {
	elb, err := r.sc.GetLoadBalancer(elbId)
	if err != nil {
		return nil, err
	}

	targets := make([]*string, len(elb.Instances))
	var instances []*string
	for i, elbInstance := range elb.Instances {
		instances, err = r.resolveInstance(*elbInstance.InstanceID)
		if err != nil {
			return nil, err
		}
		// TODO: More of this shitty assumption sauce about reservations.
		targets[i] = instances[0]
	}

	return targets, nil
}

func (r *AWSResolver) resolveInstance(instanceId string) ([]*string, error) {
	reservation, err := r.sc.GetInstance(instanceId)
	if err != nil {
		return nil, err
	}

	target := make([]*string, len(reservation.Instances))
	for i, instance := range reservation.Instances {
		target[i] = getAddrFromInstance(instance)
	}

	return target, nil
}

func (r *AWSResolver) Resolve(target *Target) ([]*string, error) {
	logger.Debug("Resolving target: %v", *target)

	switch target.Type {
	case "instance":
		return r.resolveInstance(target.Id)
	case "sg":
		return r.resolveSecurityGroup(target.Id)
	case "elb":
		return r.resolveELB(target.Name)
	}

	return []*string{}, nil
}
