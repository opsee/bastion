package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	cf "github.com/crewjam/go-cloudformation"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
)

const (
	moduleName = "hacker"
)

var (
	signalsChannel = make(chan os.Signal, 1)
)

func init() {
	signal.Notify(signalsChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

type Hacker struct {
	CustomerId                  string
	HostSecurityGroupPhysicalId string
	VpcId                       string
	ingressStackPhysicalId      string
	bastionStackPhysicalId      string
	waitTime                    time.Duration
	stackTimeoutMinutes         int64
	ec2Client                   *ec2.EC2
	cloudformationClient        *cloudformation.CloudFormation
}

func NewHacker() (*Hacker, error) {
	cfg := config.GetConfig()

	customerId := os.Getenv("CUSTOMER_ID")
	hacker := &Hacker{
		CustomerId:             customerId,
		bastionStackPhysicalId: fmt.Sprintf("opsee-stack-%s", customerId),
		waitTime:               time.Duration(time.Minute * 2),
		stackTimeoutMinutes:    int64(2),
	}

	if cfg.MetaData.VpcId == "" {
		// config previously failed to get the vpc-id.  try one more time.
		err := cfg.MetaData.Update()
		if err != nil {
			log.WithError(err).Fatal("Couldn't get vpc-id from metadata service.")
		}
		hacker.VpcId = cfg.MetaData.VpcId
	} else {
		hacker.VpcId = cfg.MetaData.VpcId
	}

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

	hacker.ec2Client = ec2.New(sess)
	hacker.cloudformationClient = cloudformation.New(sess)

	// get security group id, group name, from bastion cloudformation stack
	params := &cloudformation.DescribeStackResourcesInput{
		StackName: aws.String(hacker.bastionStackPhysicalId),
	}
	resp, err := hacker.cloudformationClient.DescribeStackResources(params)
	if err != nil {
		log.WithError(err).Fatal("Couldn't get stack resources.")
	}

	for _, resource := range resp.StackResources {
		switch *resource.LogicalResourceId {
		case "OpseeSecurityGroup":
			hacker.HostSecurityGroupPhysicalId = *resource.PhysicalResourceId
		case "OpseeBastionIngressStack":
			hacker.ingressStackPhysicalId = *resource.PhysicalResourceId
		}
	}

	return hacker, hacker.Validate()
}

// Ensures hacker has required fields retrieved from cfn and env.
func (this *Hacker) Validate() error {
	missing := []string{}
	if this.CustomerId == "" {
		missing = append(missing, "CustomerId")
	}
	if this.HostSecurityGroupPhysicalId == "" {
		missing = append(missing, "HostSecurityGroupPhysicalId")
	}
	if this.ingressStackPhysicalId == "" {
		missing = append(missing, "ingressStackPhysicalId")
	}
	if this.bastionStackPhysicalId == "" {
		missing = append(missing, "bastionStackPhysicalId")
	}

	if len(missing) > 0 {
		return fmt.Errorf("Struct missing, %s", strings.Join(missing, ", "))
	}

	return nil
}

// Returns a list of security groups for the hacker's instances vpc
func (this *Hacker) GetSecurityGroups() ([]*ec2.SecurityGroup, error) {
	output, err := this.ec2Client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(this.VpcId)},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return output.SecurityGroups, err
}

// Returns a stack template body
func (this *Hacker) GetStackTemplateBody(stackName string) (*string, error) {
	output, err := this.cloudformationClient.GetTemplate(&cloudformation.GetTemplateInput{StackName: aws.String(stackName)})
	if err != nil {
		return nil, err
	}
	return output.TemplateBody, err
}

// call create stack or update stack
func (this *Hacker) CreateOrUpdateStack(stackName string, parameters []*cloudformation.Parameter, templateBody string) (*string, error) {
	describeStacksResponse, err := this.cloudformationClient.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		log.WithError(err).Error("Failed to describe stacks.")
		return nil, err
	}
	if len(describeStacksResponse.Stacks) == 0 {
		return this.CreateStack(stackName, parameters, templateBody)
	}
	return this.UpdateStack(stackName, parameters, templateBody)
}

func (this *Hacker) CreateStack(stackName string, parameters []*cloudformation.Parameter, templateBody string) (*string, error) {
	log.Info("Attempting to create stack ", stackName)
	createStackInput := &cloudformation.CreateStackInput{
		Capabilities:     []*string{aws.String("CAPABILITY_IAM")},
		OnFailure:        aws.String("ROLLBACK"),
		Parameters:       parameters,
		StackName:        aws.String(stackName),
		Tags:             []*cloudformation.Tag{},
		TemplateBody:     aws.String(templateBody),
		TimeoutInMinutes: aws.Int64(this.stackTimeoutMinutes),
	}

	output, err := this.cloudformationClient.CreateStack(createStackInput)
	if err != nil {
		return nil, err
	}

	err = this.cloudformationClient.WaitUntilStackCreateComplete(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		log.WithError(err).Error("Stack creation failed.")
	}

	return output.StackId, err
}

func (this *Hacker) UpdateStack(stackName string, parameters []*cloudformation.Parameter, templateBody string) (*string, error) {
	log.Info("Attempting to update stack ", stackName)
	updateStackInput := &cloudformation.UpdateStackInput{
		Capabilities: []*string{aws.String("CAPABILITY_IAM")},
		Parameters:   parameters,
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(templateBody),
	}

	output, err := this.cloudformationClient.UpdateStack(updateStackInput)
	if err != nil {
		if noUpdatesAreToBePerformed(err) {
			return nil, nil
		}
		return nil, err
	}

	err = this.cloudformationClient.WaitUntilStackUpdateComplete(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		log.WithError(err).Error("Stack creation failed.")
	}

	return output.StackId, err
}

func noUpdatesAreToBePerformed(err error) bool {
	awsErr, ok := err.(awserr.RequestFailure)
	if ok && awsErr.Code() == "ValidationError" && strings.Contains(awsErr.Message(), "No updates are to be performed.") {
		return true
	}
	return false
}

// Creates new cloudformation template and updates existing bastion stack with this template. if stack does not exist, creates a new one
func (this *Hacker) Hack() (*string, error) {
	securityGroups, err := this.GetSecurityGroups()
	if err != nil {
		log.WithError(err).Error("Couldn't retrieve security groups")
		return nil, err
	}

	template := cf.NewTemplate()
	template.Description = "Listing of bastion security-group ingress rules."
	template.Parameters["BastionSecurityGroupId"] = &cf.Parameter{
		Description: "Bastion's security group id.",
		Type:        "String",
	}

	parameters := []*cloudformation.Parameter{
		&cloudformation.Parameter{
			ParameterKey:   aws.String("BastionSecurityGroupId"),
			ParameterValue: aws.String(this.HostSecurityGroupPhysicalId),
		},
	}

	for i, securityGroup := range securityGroups {
		log.Debug("Adding security Group: ", *securityGroup.GroupId)
		resourceName := fmt.Sprintf("OpseeIngressRule%d", i)
		template.AddResource(resourceName, cf.EC2SecurityGroupIngress{
			IpProtocol:            cf.String("tcp"),
			FromPort:              cf.Integer(0),
			ToPort:                cf.Integer(65535),
			SourceSecurityGroupId: cf.Ref("BastionSecurityGroupId").String(),
			GroupId:               cf.String(*securityGroup.GroupId),
		})
	}

	templateBody, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		log.WithError(err).Error("Failed to marshal template body.")
		return nil, err
	}

	stackName, err := this.CreateOrUpdateStack(this.ingressStackPhysicalId, parameters, string(templateBody))
	if err != nil {
		log.WithError(err).Error("Failed to create or update stack.")
		return nil, err
	}

	// If nothing was updated, don't poll the stack state
	// if we do, it could look like a failure
	if err == nil && stackName == nil {
		log.Info("No updates are to be performed.")
		return nil, nil
	}

	return stackName, nil
}

func main() {
	hacker, err := NewHacker()
	if err != nil {
		log.WithError(err).Fatal("Error starting hacker.")
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		log.Fatal(err.Error())
	}
	beatChan := heart.Beat()

	// hack forever
	go func() {
		log.Info("Started hacker.")
		for {
			t := time.Now()
			log.Info("Hacking")
			_, err := hacker.Hack()
			if err != nil {
				log.WithError(err).Error("Couldn't update the stack.")
			}
			if wait := hacker.waitTime - time.Since(t); wait > time.Millisecond {
				log.Info("Waiting ", wait)
				time.Sleep(wait)
			}
		}
	}()

	for {
		select {
		case s := <-signalsChannel:
			switch s {
			case syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT:
				log.Info("Received signal ", s, ". Stopping.")
				os.Exit(0)
			}
		case beatErr := <-beatChan:
			log.WithError(beatErr).Error("Heartbeat error.")
		}
	}
}
