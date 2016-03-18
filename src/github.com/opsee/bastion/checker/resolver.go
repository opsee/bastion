package checker

import (
	"fmt"
	"net"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/opsee/basic/schema"
	"github.com/opsee/bastion/config"

	log "github.com/Sirupsen/logrus"
)

type Resolver interface {
	Resolve(*schema.Target) ([]*schema.Target, error)
}

// TODO: The resolver should not query the EC2Scanner directly, but
// should go through the instance/group cache instead.
type AWSResolver struct {
	ec2Client *ec2.EC2
	elbClient *elb.ELB
	VpcId     string
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
	})

	resolver := &AWSResolver{
		ec2Client: ec2.New(sess),
		elbClient: elb.New(sess),
		VpcId:     cfg.MetaData.VpcId,
	}

	return resolver
}

func (this *AWSResolver) resolveSecurityGroup(sgId string) ([]*schema.Target, error) {
	var grs []*string = []*string{aws.String(sgId)}
	var reservations []*ec2.Reservation

	filters := []*ec2.Filter{
		{
			Name:   aws.String("vpc-id"),
			Values: []*string{aws.String(this.VpcId)},
		},
		{
			Name:   aws.String("instance.group-id"),
			Values: grs,
		},
	}

	err := this.ec2Client.DescribeInstancesPages(&ec2.DescribeInstancesInput{Filters: filters}, func(resp *ec2.DescribeInstancesOutput, lastPage bool) bool {
		for _, res := range resp.Reservations {
			reservations = append(reservations, res)
		}
		if lastPage {
			return false
		}
		return true
	})

	if err != nil {
		return nil, err
	}

	log.Debug("reservations: %v", reservations)

	var targets []*schema.Target
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			targets = append(targets, &schema.Target{
				Id:      *instance.InstanceId,
				Type:    "instance",
				Address: getAddrFromInstance(instance),
			})
		}
	}
	return targets, nil
}

func (this *AWSResolver) resolveELB(elbId string) ([]*schema.Target, error) {
	input := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []*string{aws.String(elbId)},
	}

	resp, err := this.elbClient.DescribeLoadBalancers(input)
	if err != nil {
		return nil, err
	}

	elb := resp.LoadBalancerDescriptions[0]
	if aws.StringValue(elb.VPCId) != this.VpcId {
		return nil, fmt.Errorf("LoadBalancer not found with vpc id = %s", this.VpcId)
	}

	targets := make([]*schema.Target, len(elb.Instances))
	for i, elbInstance := range elb.Instances {
		t, err := this.resolveInstance(*elbInstance.InstanceId)
		if err != nil {
			return nil, err
		}
		targets[i] = t[0]
	}

	return targets, nil
}

func (this *AWSResolver) resolveInstance(instanceId string) ([]*schema.Target, error) {
	resp, err := this.ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(this.VpcId)},
			},
		},
		InstanceIds: []*string{&instanceId},
	})

	if err != nil {
		if len(resp.Reservations) > 1 {
			return nil, fmt.Errorf("Received multiple reservations for instance id: %v, %v", instanceId, resp)
		}
		return nil, err
	}

	// InstanceId to Reservation mappings are 1-to-1
	reservation := resp.Reservations[0]

	target := make([]*schema.Target, len(reservation.Instances))
	for i, instance := range reservation.Instances {
		target[i] = &schema.Target{
			Type:    "instance",
			Id:      instanceId,
			Address: getAddrFromInstance(instance),
		}
	}

	return target, nil
}

func (this *AWSResolver) resolveHost(host string) ([]*schema.Target, error) {
	log.Debugf("resolving host: %s", host)

	ips, err := net.LookupIP(host)
	if err != nil {
		log.WithError(err).Errorf("error resolving host: %s", host)
		return nil, err
	}

	target := make([]*schema.Target, 0, len(ips))
	for _, ip := range ips {
		// ipv4 only
		if ip.To4() != nil {
			target = append(target, &schema.Target{
				Type:    "host",
				Id:      host,
				Address: ip.String(),
			})
		}
	}

	return target, nil
}

func (this *AWSResolver) Resolve(target *schema.Target) ([]*schema.Target, error) {
	log.Debug("Resolving target: %v", *target)

	switch target.Type {
	case "sg":
		return this.resolveSecurityGroup(target.Id)
	case "elb":
		// TODO(greg): We should probably handle this kind of thing better. This
		// came to pass, because ELBs don't have IDs, they only have names.
		// HOWEVER, somewhere along the line, the ELB Name is saved as its ID.
		// Because on the Opsee side, every resource needs an ID. So, when the
		// request is made to create the check, lo and behold, the ELB Target object
		// ends up with an ID and no Name. So we account for that here.
		if target.Name != "" {
			return this.resolveELB(target.Name)
		}
		if target.Id != "" {
			return this.resolveELB(target.Id)
		}
		return nil, fmt.Errorf("Invalid target: %s", target.String())
	case "instance":
		return this.resolveInstance(target.Id)
	case "host":
		return this.resolveHost(target.Id)
	}

	return nil, fmt.Errorf("Unable to resolve target: %s", target)
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
