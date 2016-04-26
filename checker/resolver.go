package checker

import (
	"fmt"
	"net"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/opsee/basic/schema"
	opsee_aws_ec2 "github.com/opsee/basic/schema/aws/ec2"
	opsee "github.com/opsee/basic/service"
	opsee_types "github.com/opsee/protobuf/opseeproto/types"
	"github.com/opsee/bastion/config"

	log "github.com/Sirupsen/logrus"
)

var (
	DefaultResponseCacheTTL = time.Minute * 5
	MaxEC2Instances = 200 // XXX(mike) but is it enough?
)

type Resolver interface {
	Resolve(ctx context.Context, *schema.Target) ([]*schema.Target, error)
}

// TODO: The resolver should not query the EC2Scanner directly, but
// should go through the instance/group cache instead.
type AWSResolver struct {
	BezosClient *opsee.BezosClient
	VpcId       string
	Region string
	User *schema.User
}

func NewResolver(bezos *opsee.BezosClient, cfg *config.Config) Resolver {
	metaData, err := cfg.AWS.MetaData()
	if err != nil {
		log.WithError(err).Fatal("Couldn't get metadata from global config.")
	}

	user := &schema.User{
		CustomerID: cfg.CustomerID,
		Email: cfg.CustomerEmail,
		Admin: false,
	}

	resolver := &AWSResolver{
		BezosClient: bezos,
		VpcId:       metaData.VpcId,
		Region: metaData.Region,
		User: user,
	}

	return resolver
}

func (this *AWSResolver) resolveSecurityGroup(ctx context.Context, sgId string) ([]*schema.Target, error) {
	input := &opsee_aws_ec2.DescribeInstancesInput{
		Filters: []*opsee_aws_ec2.Filter{
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

func (this *AWSResolver) resolveEC2Instances(ctx context.Context, instanceIds ...string) ([]*schema.Target, error) {
	ids := []*string{}
	for _, id := range instanceIds {
		ids = append(ids, aws.String(id))
	}

	input := &opsee_aws_ec2.DescribeInstancesInput{
		Filters: []*opsee_aws_ec2.Filter{
			&opsee_aws_ec2.Filter{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(this.VpcId)},
			},
		},
		InstanceIds: ids,
		MaxResults: MaxEC2Instances,
	}

	return this.resolveEC2InstancesWithInput(ctx, input)
}

func (this *AWSResolver) resolveEC2InstancesWithInput(ctx context.Context, input *opsee_aws_ec2.DescribeInstancesInput) ([]*schema.Target, error) {
	var reservations []*opsee_aws_ec2.Reservation

	timestamp := &opsee_types.Timestamp{}
	resp, err := this.BezosClient.Get(
		ctx,
		&opsee.BezosRequest{
			User: this.User,
			Region: this.Region,
			VpcId: this.VpcId,
			MaxAge: timestamp.Scan(time.Now().UTC().Sub(DefaultResponseCacheTTL)),
			Input: &opsee.BezosRequest_Ec2_DescribeInstancesInput{input}
		})
	if err != nil {
		return nil, err
	}

	output := resp.GetEc2_DescribeInstancesOutput()
	if output == nil {
		return nil, fmt.Errorf("error decoding aws response")
	}


	for _, output:= range resp.Reservations {
		reservations = append(reservations, res)
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

func (this *AWSResolver) resolveASGs(ctx context.Context, asgNames ...string) ([]*schema.Target, error) {
	names := []*string{}
	for _, name := range asgNames {
		names = append(names, aws.String(name))
	}

	timestamp := &opsee_types.Timestamp{}
	maxAge := timestamp.Scan(time.Now().UTC().Sub(DefaultResponseCacheTTL)),
	resp, err := this.BezosClient.Get(
		ctx,
		&opsee.BezosRequest{

		}
	)

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

func (this *AWSResolver) resolveELBs(ctx context.Context, elbNames ...string) ([]*schema.Target, error) {
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
func (this *AWSResolver) resolveDBInstance(ctx context.Context, instanceId string) ([]*schema.Target, error) {
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

func (this *AWSResolver) Resolve(ctx context.Context, target *schema.Target) ([]*schema.Target, error) {
	log.Debug("Resolving target: %v", *target)

	switch target.Type {
	case "sg":
		return this.resolveSecurityGroup(ctx, target.Id)
	case "elb":
		// TODO(greg): We should probably handle this kind of thing better. This
		// came to pass, because ELBs don't have IDs, they only have names.
		// HOWEVER, somewhere along the line, the ELB Name is saved as its ID.
		// Because on the Opsee side, every resource needs an ID. So, when the
		// request is made to create the check, lo and behold, the ELB Target object
		// ends up with an ID and no Name. So we account for that here.
		if target.Name != "" {
			return this.resolveELBs(ctx, target.Name)
		}
		if target.Id != "" {
			return this.resolveELBs(ctx, target.Id)
		}
		return nil, fmt.Errorf("Invalid target: %s", target.String())
	case "asg":
		if target.Name != "" {
			return this.resolveASGs(ctx, target.Name)
		}
		if target.Id != "" {
			return this.resolveASGs(ctx, target.Id)
		}
		return nil, fmt.Errorf("Invalid target: %s", target.String())
	case "instance":
		return this.resolveEC2Instances(ctx, target.Id)
	case "dbinstance":
		return this.resolveDBInstance(ctx, target.Id)
	case "host":
		return this.resolveHost(target.Id)
	}

	return nil, fmt.Errorf("Unable to resolve target: %s", target)
}

// TODO: In some cases this won't be so easy.
// TODO: Also, god help us if a reservation contains more than one
// instance
func getAddrFromInstance(instance *opsee_aws_ec2.Instance) string {
	var addr *string
	if instance.PrivateIpAddress != nil {
		addr = instance.PrivateIpAddress
	} else if instance.PublicIpAddress != nil {
		addr = instance.PublicIpAddress
	}

	return *addr
}
