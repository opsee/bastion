package checker

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/opsee/awscan"
	"github.com/opsee/bastion/config"

	log "github.com/Sirupsen/logrus"
)

type Resolver interface {
	Resolve(*Target) ([]*Target, error)
}

// TODO: The resolver should not query the EC2Scanner directly, but
// should go through the instance/group cache instead.
type AWSResolver struct {
	sc awscan.EC2Scanner
}

func NewResolver(cfg *config.Config) Resolver {
	creds := credentials.NewChainCredentials(
		[]credentials.Provider{
			&ec2rolecreds.EC2RoleProvider{
				Client: ec2metadata.New(session.New()),
			},
			&credentials.EnvProvider{},
		},
	)

	sess := session.New(&aws.Config{
		Credentials: creds,
		Region:      aws.String(cfg.MetaData.Region),
		MaxRetries:  aws.Int(11),
	})

	resolver := &AWSResolver{
		sc: awscan.NewScanner(sess, cfg.MetaData.VPCID),
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

	log.Debug("reservations: %v", reservations)

	var targets []*Target
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			targets = append(targets, &Target{
				Id:      *instance.InstanceId,
				Type:    "instance",
				Address: getAddrFromInstance(instance),
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
		t, err := r.resolveInstance(*elbInstance.InstanceId)
		if err != nil {
			return nil, err
		}
		targets[i] = t[0]
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
			Type:    "instance",
			Id:      instanceId,
			Address: getAddrFromInstance(instance),
		}
	}

	return target, nil
}

func (r *AWSResolver) Resolve(target *Target) ([]*Target, error) {
	log.Debug("Resolving target: %v", *target)

	switch target.Type {
	case "sg":
		return r.resolveSecurityGroup(target.Id)
	case "elb":
		// TODO(greg): We should probably handle this kind of thing better. This
		// came to pass, because ELBs don't have IDs, they only have names.
		// HOWEVER, somewhere along the line, the ELB Name is saved as its ID.
		// Because on the Opsee side, every resource needs an ID. So, when the
		// request is made to create the check, lo and behold, the ELB Target object
		// ends up with an ID and no Name. So we account for that here.
		if target.Name != "" {
			return r.resolveELB(target.Name)
		}
		if target.Id != "" {
			return r.resolveELB(target.Id)
		}
		return nil, fmt.Errorf("Invalid target: %s", target.String())
	case "instance":
		return r.resolveInstance(target.Id)
	}

	return nil, fmt.Errorf("Unable to resolve target: %s", target)
}
