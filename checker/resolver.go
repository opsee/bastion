package checker

import (
	"fmt"
	"net"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/rds"
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
	rdsClient *rds.RDS
	asgClient *autoscaling.AutoScaling
	VpcId     string
}

func NewResolver(cfg *config.Config) Resolver {
	sess, err := cfg.AWS.Session()
	if err != nil {
		log.WithError(err).Fatal("Couldn't get aws session from global config.")
	}

	metaData, err := cfg.AWS.MetaData()
	if err != nil {
		log.WithError(err).Fatal("Couldn't get metadata from global config.")
	}

	resolver := &AWSResolver{
		ec2Client: ec2.New(sess),
		elbClient: elb.New(sess),
		rdsClient: rds.New(sess),
		asgClient: autoscaling.New(sess),
		VpcId:     metaData.VpcId,
	}

	return resolver
}

func (this *AWSResolver) resolveSecurityGroup(sgId string) ([]*schema.Target, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(this.VpcId)},
			},
			{
				Name:   aws.String("instance.group-id"),
				Values: []*string{aws.String(sgId)},
			},
		},
	}

	return this.resolveEC2InstancesWithInput(input)
}

func (this *AWSResolver) resolveEC2Instances(instanceIds ...string) ([]*schema.Target, error) {
	ids := []*string{}
	for _, id := range instanceIds {
		ids = append(ids, aws.String(id))
	}
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(this.VpcId)},
			},
		},
		InstanceIds: ids,
	}

	return this.resolveEC2InstancesWithInput(input)
}

func (this *AWSResolver) resolveEC2InstancesWithInput(input *ec2.DescribeInstancesInput) ([]*schema.Target, error) {
	var reservations []*ec2.Reservation
	err := this.ec2Client.DescribeInstancesPages(input, func(resp *ec2.DescribeInstancesOutput, lastPage bool) bool {
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

func (this *AWSResolver) resolveASGs(asgNames ...string) ([]*schema.Target, error) {
	names := []*string{}
	for _, name := range asgNames {
		names = append(names, aws.String(name))
	}

	resp, err := this.asgClient.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: names,
	})
	if err != nil {
		return nil, err
	}

	instanceIds := []string{}
	for _, gr := range resp.AutoScalingGroups {
		for _, instance := range gr.Instances {
			if aws.StringValue(instance.LifecycleState) == autoscaling.LifecycleStateInService {
				instanceIds = append(instanceIds, aws.StringValue(instance.InstanceId))
			}
		}
	}
	return this.resolveEC2Instances(instanceIds...)
}

func (this *AWSResolver) resolveELBs(elbNames ...string) ([]*schema.Target, error) {
	names := []*string{}
	for _, name := range elbNames {
		names = append(names, aws.String(name))
	}

	input := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: names,
	}

	resp, err := this.elbClient.DescribeLoadBalancers(input)
	if err != nil {
		return nil, err
	}

	elb := resp.LoadBalancerDescriptions[0]
	if aws.StringValue(elb.VPCId) != this.VpcId {
		return nil, fmt.Errorf("LoadBalancer not found with vpc id = %s", this.VpcId)
	}

	instanceIds := []string{}
	for _, elbInstance := range elb.Instances {
		instanceIds = append(instanceIds, aws.StringValue(elbInstance.InstanceId))
	}
	return this.resolveEC2Instances(instanceIds...)
}

// in case we need it some day
func (this *AWSResolver) resolveDBInstance(instanceId string) ([]*schema.Target, error) {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceId),
	}

	resp, err := this.rdsClient.DescribeDBInstances(input)
	if err != nil {
		return nil, err
	}

	target := make([]*schema.Target, len(resp.DBInstances))
	for i, instance := range resp.DBInstances {
		target[i] = &schema.Target{
			Type:    "dbinstance",
			Id:      instanceId,
			Address: *instance.Endpoint.Address, // the instances actual http address
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
			return this.resolveELBs(target.Name)
		}
		if target.Id != "" {
			return this.resolveELBs(target.Id)
		}
		return nil, fmt.Errorf("Invalid target: %s", target.String())
	case "asg":
		if target.Name != "" {
			return this.resolveASGs(target.Name)
		}
		if target.Id != "" {
			return this.resolveASGs(target.Id)
		}
		return nil, fmt.Errorf("Invalid target: %s", target.String())
	case "instance":
		return this.resolveEC2Instances(target.Id)
	case "dbinstance":
		return this.resolveDBInstance(target.Id)
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
