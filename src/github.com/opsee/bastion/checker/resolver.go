package checker

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/scanner"
)

type Resolver interface {
	Resolve(*Target) ([]*Target, error)
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
func getAddrFromInstance(instance *ec2.Instance) string {
	var addr *string
	if instance.PrivateIpAddress != nil {
		addr = instance.PrivateIpAddress
	} else if instance.PublicIpAddress != nil {
		addr = instance.PublicIpAddress
	}

	return *addr
}

func (r *AWSResolver) resolveSecurityGroup(sgid string) ([]*Target, error) {
	reservations, err := r.sc.ScanSecurityGroupInstances(sgid)
	if err != nil {
		return nil, err
	}

	logger.Debug("reservations: %v", reservations)

	var targets []*Target
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			targets = append(targets, &Target{
				Id:   *instance.InstanceId,
				Type: "instance",
			})
		}
	}
	return targets, nil
}

func (r *AWSResolver) resolveELB(elbId string) ([]*Target, error) {
	elb, err := r.sc.GetLoadBalancer(elbId)
	if err != nil {
		return nil, err
	}

	targets := make([]*Target, len(elb.Instances))
	for i, elbInstance := range elb.Instances {
		targets[i] = &Target{
			Id:   *elbInstance.InstanceId,
			Type: "instance",
		}
	}

	return targets, nil
}

func (r *AWSResolver) resolveInstance(instanceId string) ([]*Target, error) {
	reservation, err := r.sc.GetInstance(instanceId)
	if err != nil {
		return nil, err
	}

	target := make([]*Target, len(reservation.Instances))
	for i, instance := range reservation.Instances {
		target[i] = &Target{
			Type: "ip",
			Id:   getAddrFromInstance(instance),
		}
	}

	return target, nil
}

func (r *AWSResolver) Resolve(target *Target) ([]*Target, error) {
	logger.Debug("Resolving target: %v", *target)

	switch target.Type {
	case "sg":
		return r.resolveSecurityGroup(target.Id)
	case "elb":
		return r.resolveELB(target.Name)
	case "instance":
		return r.resolveInstance(target.Id)
	case "ip":
		return []*Target{target}, nil
	}

	return []*Target{}, nil
}
