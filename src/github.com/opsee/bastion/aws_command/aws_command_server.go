package aws_command

import (
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/opsee/bastion/config"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	awsEC2Client *ec2.EC2
)

func init() {
	cfg := config.GetConfig()
	sess, err := cfg.AWS.Session()
	if err != nil {
		log.WithError(err).Fatal("Couldn't get AWS Session")
	}
	awsEC2Client = ec2.New(sess)
}

type AWSCommander struct {
	Port       int
	grpcServer *grpc.Server
}

func (s *AWSCommander) Start() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", s.Port))
	if err != nil {
		return err
	}

	RegisterEc2Server(s.grpcServer, s)
	go s.grpcServer.Serve(listen)

	return nil
}

func NewAWSCommander() *AWSCommander {
	return &AWSCommander{
		grpcServer: grpc.NewServer(),
	}
}

func (s *AWSCommander) StopInstances(ctx context.Context, in *StopInstancesRequest) (*StopInstancesResult, error) {

	stopInstancesInput := &ec2.StopInstancesInput{
		DryRun:      aws.Bool(in.DryRun),
		Force:       aws.Bool(in.Force),
		InstanceIds: aws.StringSlice(in.InstanceIds),
	}
	resp, err := awsEC2Client.StopInstances(stopInstancesInput)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			log.Error("Error:", awsErr)
		} else {
			log.Error("Error:", err.Error())
		}
	}

	instanceStateChanges := make([]*InstanceStateChange, len(resp.StoppingInstances))
	for i := range instanceStateChanges {
		instanceStateChanges[i] = &InstanceStateChange{
			InstanceId:    *resp.StoppingInstances[i].InstanceId,
			CurrentState:  &InstanceState{Code: *resp.StoppingInstances[i].CurrentState.Code, Name: *resp.StoppingInstances[i].CurrentState.Name},
			PreviousState: &InstanceState{Code: *resp.StoppingInstances[i].PreviousState.Code, Name: *resp.StoppingInstances[i].PreviousState.Name},
		}
	}

	stopInstancesResult := &StopInstancesResult{
		StoppingInstances: instanceStateChanges,
	}

	return stopInstancesResult, err
}

func (s *AWSCommander) StartInstances(ctx context.Context, in *StartInstancesRequest) (*StartInstancesResult, error) {

	input := &ec2.StartInstancesInput{
		AdditionalInfo: aws.String(in.AdditionalInfo),
		DryRun:         aws.Bool(in.DryRun),
		InstanceIds:    aws.StringSlice(in.InstanceIds),
	}

	resp, err := awsEC2Client.StartInstances(input)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			log.Error("Error:", awsErr)
		} else {
			log.Error("Error:", err.Error())
		}
	}

	instanceStateChanges := make([]*InstanceStateChange, len(resp.StartingInstances))
	for i := range instanceStateChanges {
		instanceStateChanges[i] = &InstanceStateChange{
			InstanceId:    *resp.StartingInstances[i].InstanceId,
			CurrentState:  &InstanceState{Code: *resp.StartingInstances[i].CurrentState.Code, Name: *resp.StartingInstances[i].CurrentState.Name},
			PreviousState: &InstanceState{Code: *resp.StartingInstances[i].PreviousState.Code, Name: *resp.StartingInstances[i].PreviousState.Name},
		}
	}

	startInstancesResult := &StartInstancesResult{
		StartingInstances: instanceStateChanges,
	}

	return startInstancesResult, err
}

func (s *AWSCommander) RebootInstances(ctx context.Context, in *RebootInstancesRequest) (*RebootInstancesResult, error) {

	input := &ec2.RebootInstancesInput{
		DryRun:      aws.Bool(in.DryRun),
		InstanceIds: aws.StringSlice(in.InstanceIds),
	}

	resp, err := awsEC2Client.RebootInstances(input)

	log.Debug(resp)
	rebootInstancesResult := &RebootInstancesResult{Err: ""}

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			rebootInstancesResult = &RebootInstancesResult{Err: awsErr.Message()}
			log.Error("Error:", awsErr)
		} else {
			rebootInstancesResult = &RebootInstancesResult{Err: err.Error()}
			log.Error("Error:", err.Error())
		}
	}

	return rebootInstancesResult, err
}
