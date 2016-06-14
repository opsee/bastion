package checker

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/opsee/basic/schema"
	opsee_aws_autoscaling "github.com/opsee/basic/schema/aws/autoscaling"
	opsee_aws_ec2 "github.com/opsee/basic/schema/aws/ec2"
	opsee_aws_ecs "github.com/opsee/basic/schema/aws/ecs"
	opsee_aws_elb "github.com/opsee/basic/schema/aws/elb"
	opsee_aws_rds "github.com/opsee/basic/schema/aws/rds"
	opsee "github.com/opsee/basic/service"
	"github.com/opsee/bastion/config"
	opsee_types "github.com/opsee/protobuf/opseeproto/types"
	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
)

var (
	DefaultResponseCacheTTL = time.Second * time.Duration(5)
)

/* A Respolver resolves a target into an array of targets that are
 * directly addressable by a bastion component.
 *
 * It is important to note, that Target IDs should be unique across
 * a set of responses. This makes them
 */
type Resolver interface {
	Resolve(context.Context, *schema.Target) ([]*schema.Target, error)
}

// TODO: The resolver should not query the EC2Scanner directly, but
// should go through the instance/group cache instead.
type AWSResolver struct {
	BezosClient opsee.BezosClient
	VpcId       string
	Region      string
	User        *schema.User
}

func NewResolver(bezos opsee.BezosClient, cfg *config.Config) Resolver {
	metaData, err := cfg.AWS.MetaData()
	if err != nil {
		log.WithError(err).Fatal("Couldn't get metadata from global config.")
	}

	user := &schema.User{
		Id:         1,
		Verified:   true,
		Active:     true,
		CustomerId: cfg.CustomerId,
		Email:      cfg.CustomerEmail,
		Admin:      false,
	}

	resolver := &AWSResolver{
		BezosClient: bezos,
		VpcId:       metaData.VpcId,
		Region:      metaData.Region,
		User:        user,
	}

	return resolver
}

func (this *AWSResolver) resolveSecurityGroup(ctx context.Context, sgId string) ([]*schema.Target, error) {
	input := &opsee_aws_ec2.DescribeInstancesInput{
		Filters: []*opsee_aws_ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{this.VpcId},
			},
			{
				Name:   aws.String("instance.group-id"),
				Values: []string{sgId},
			},
		},
	}

	return this.resolveEC2InstancesWithInput(ctx, input)
}

func (this *AWSResolver) resolveEC2Instances(ctx context.Context, instanceIds ...string) ([]*schema.Target, error) {
	ids := []string{}
	for _, id := range instanceIds {
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return []*schema.Target{}, nil
	}

	input := &opsee_aws_ec2.DescribeInstancesInput{
		Filters: []*opsee_aws_ec2.Filter{
			&opsee_aws_ec2.Filter{
				Name:   aws.String("vpc-id"),
				Values: []string{this.VpcId},
			},
		},
		InstanceIds: ids,
	}

	return this.resolveEC2InstancesWithInput(ctx, input)
}

func (this *AWSResolver) resolveEC2InstancesWithInput(ctx context.Context, input *opsee_aws_ec2.DescribeInstancesInput) ([]*schema.Target, error) {
	timestamp := &opsee_types.Timestamp{}
	timestamp.Scan(time.Now().UTC().Add(DefaultResponseCacheTTL * -1))
	resp, err := this.BezosClient.Get(
		ctx,
		&opsee.BezosRequest{
			User:   this.User,
			Region: this.Region,
			VpcId:  this.VpcId,
			MaxAge: timestamp,
			Input:  &opsee.BezosRequest_Ec2_DescribeInstancesInput{input},
		})
	if err != nil {
		return nil, err
	}

	output := resp.GetEc2_DescribeInstancesOutput()
	if output == nil {
		return nil, fmt.Errorf("error decoding aws response")
	}

	var targets []*schema.Target
	for _, res := range output.Reservations {
		for _, instance := range res.Instances {
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
	names := []string{}
	for _, name := range asgNames {
		names = append(names, name)
	}

	input := &opsee_aws_autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: names,
	}

	maxAge := &opsee_types.Timestamp{}
	maxAge.Scan(time.Now().UTC().Add(DefaultResponseCacheTTL * -1))
	resp, err := this.BezosClient.Get(
		ctx,
		&opsee.BezosRequest{
			User:   this.User,
			Region: this.Region,
			VpcId:  this.VpcId,
			MaxAge: maxAge,
			Input:  &opsee.BezosRequest_Autoscaling_DescribeAutoScalingGroupsInput{input},
		})
	if err != nil {
		return nil, err
	}

	output := resp.GetAutoscaling_DescribeAutoScalingGroupsOutput()
	if output == nil {
		return nil, fmt.Errorf("error decoding aws response")
	}

	instanceIds := []string{}
	for _, gr := range output.AutoScalingGroups {
		for _, instance := range gr.Instances {
			if aws.StringValue(instance.LifecycleState) == autoscaling.LifecycleStateInService {
				instanceIds = append(instanceIds, aws.StringValue(instance.InstanceId))
			}
		}
	}
	return this.resolveEC2Instances(ctx, instanceIds...)
}

func (this *AWSResolver) resolveELBs(ctx context.Context, elbNames ...string) ([]*schema.Target, error) {
	names := []string{}
	for _, name := range elbNames {
		names = append(names, name)
	}

	input := &opsee_aws_elb.DescribeLoadBalancersInput{
		LoadBalancerNames: names,
	}

	maxAge := &opsee_types.Timestamp{}
	maxAge.Scan(time.Now().UTC().Add(DefaultResponseCacheTTL * -1))
	resp, err := this.BezosClient.Get(
		ctx,
		&opsee.BezosRequest{
			User:   this.User,
			Region: this.Region,
			VpcId:  this.VpcId,
			MaxAge: maxAge,
			Input:  &opsee.BezosRequest_Elb_DescribeLoadBalancersInput{input},
		})
	if err != nil {
		return nil, err
	}

	output := resp.GetElb_DescribeLoadBalancersOutput()
	if output == nil {
		return nil, err
	}

	elb := output.LoadBalancerDescriptions[0]
	if aws.StringValue(elb.VPCId) != this.VpcId {
		return nil, fmt.Errorf("LoadBalancer not found with vpc id = %s", this.VpcId)
	}

	instanceIds := []string{}
	for _, elbInstance := range elb.Instances {
		instanceIds = append(instanceIds, aws.StringValue(elbInstance.InstanceId))
	}
	return this.resolveEC2Instances(ctx, instanceIds...)
}

// in case we need it some day
func (this *AWSResolver) resolveDBInstance(ctx context.Context, instanceId string) ([]*schema.Target, error) {
	input := &opsee_aws_rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceId),
	}

	maxAge := &opsee_types.Timestamp{}
	maxAge.Scan(time.Now().UTC().Add(DefaultResponseCacheTTL * -1))
	resp, err := this.BezosClient.Get(
		ctx,
		&opsee.BezosRequest{
			User:   this.User,
			Region: this.Region,
			VpcId:  this.VpcId,
			MaxAge: maxAge,
			Input:  &opsee.BezosRequest_Rds_DescribeDBInstancesInput{input},
		})
	if err != nil {
		return nil, err
	}

	output := resp.GetRds_DescribeDBInstancesOutput()
	if output == nil {
		return nil, fmt.Errorf("error decoding aws response")
	}

	target := make([]*schema.Target, len(output.DBInstances))
	for i, instance := range output.DBInstances {
		target[i] = &schema.Target{
			Type:    "dbinstance",
			Id:      instanceId,
			Address: *instance.Endpoint.Address, // the instances actual http address
		}
	}

	return target, nil
}

func (this *AWSResolver) resolveECSService(ctx context.Context, id string) ([]*schema.Target, error) {
	t := strings.Split(id, "\x00")
	if len(t) < 2 {
		return nil, fmt.Errorf("Invalid ECS Service target.")
	}

	cluster_name := t[0]
	service_name := t[1]

	ltInput := &opsee_aws_ecs.ListTasksInput{
		Cluster:     aws.String(cluster_name),
		ServiceName: aws.String(service_name),
	}

	maxAge := &opsee_types.Timestamp{}
	maxAge.Scan(time.Now().UTC().Add(DefaultResponseCacheTTL * -1))
	bltResponse, err := this.BezosClient.Get(
		ctx,
		&opsee.BezosRequest{
			User:   this.User,
			Region: this.Region,
			VpcId:  this.VpcId,
			Input:  &opsee.BezosRequest_Ecs_ListTasksInput{ltInput},
		},
	)
	if err != nil {
		return nil, err
	}

	ltOutput := bltResponse.GetEcs_ListTasksOutput()
	if ltOutput == nil {
		return nil, fmt.Errorf("error decoding aws response")
	}

	var tasks []string
	for _, t := range ltOutput.TaskArns {
		tasks = append(tasks, t)
	}

	dtInput := &opsee_aws_ecs.DescribeTasksInput{
		Tasks:   tasks,
		Cluster: aws.String(cluster_name),
	}

	bdtResponse, err := this.BezosClient.Get(
		ctx,
		&opsee.BezosRequest{
			User:   this.User,
			Region: this.Region,
			VpcId:  this.VpcId,
			Input:  &opsee.BezosRequest_Ecs_DescribeTasksInput{dtInput},
		},
	)
	if err != nil {
		return nil, err
	}

	bdtOutput := bdtResponse.GetEcs_DescribeTasksOutput()
	if bdtOutput == nil {
		return nil, fmt.Errorf("error decoding aws response")
	}

	var ci []string
	for _, t := range bdtOutput.Tasks {
		ci = append(ci, aws.StringValue(t.ContainerInstanceArn))
	}

	dciInput := &opsee_aws_ecs.DescribeContainerInstancesInput{
		ContainerInstances: ci,
	}

	dciResp, err := this.BezosClient.Get(
		ctx,
		&opsee.BezosRequest{
			User:   this.User,
			Region: this.Region,
			VpcId:  this.VpcId,
			Input:  &opsee.BezosRequest_Ecs_DescribeContainerInstancesInput{dciInput},
		},
	)
	if err != nil {
		return nil, err
	}

	dciOutput := dciResp.GetEcs_DescribeContainerInstancesOutput()
	if dciOutput == nil {
		return nil, fmt.Errorf("error decoding aws response")
	}

	instances := make([]string, 0, len(ci))
	for i, inst := range dciOutput.ContainerInstances {
		instances[i] = aws.StringValue(inst.Ec2InstanceId)
	}

	return this.resolveEC2InstancesWithInput(ctx, &opsee_aws_ec2.DescribeInstancesInput{
		Filters: []*opsee_aws_ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{this.VpcId},
			},
		},
		InstanceIds: instances,
	})
}

func (this *AWSResolver) resolveHost(hostType, host string) ([]*schema.Target, error) {
	log.Debugf("resolving %s: %s", hostType, host)

	ips, err := net.LookupIP(host)
	if err != nil {
		log.WithError(err).Errorf("error resolving host: %s", host)
		return nil, err
	}

	target := make([]*schema.Target, 0, len(ips))
	for _, ip := range ips {
		// ipv4 only
		if ip.To4() != nil {
			ipstr := ip.String()
			target = append(target, &schema.Target{
				// name is very important, please leave.
				// it's used by the http check runner to determine hostname for TLS
				Name:    host,
				Type:    hostType,
				Id:      ipstr,
				Address: ipstr,
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
		if target.Id != "" {
			return this.resolveASGs(ctx, target.Id)
		}
		return nil, fmt.Errorf("Invalid target: %s", target.String())
	case "instance":
		return this.resolveEC2Instances(ctx, target.Id)
	case "dbinstance":
		return this.resolveDBInstance(ctx, target.Id)
	case "ecs_service":
		return this.resolveECSService(ctx, target.Id)
	case "host":
		return this.resolveHost("host", target.Id)
	case "external_host":
		return this.resolveHost("external_host", target.Id)
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
