package scanner

import(
		// "fmt"
		"github.com/awslabs/aws-sdk-go/aws"
		"github.com/awslabs/aws-sdk-go/gen/ec2"
		"github.com/awslabs/aws-sdk-go/gen/elb"
		"github.com/awslabs/aws-sdk-go/gen/rds"
		"bastion/credentials"
		// "github.com/awslabs/aws-sdk-go/gen/autoscaling"
    "github.com/op/go-logging"
)

var (
    log = logging.MustGetLogger("bastion.json-tcp")
    logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}")
)

func init() {
    logging.SetLevel(logging.INFO, "json-tcp")
    logging.SetFormatter(logFormat)
}

type EC2Scanner interface {
	ScanSecurityGroups() ([]ec2.SecurityGroup, error)
	ScanSecurityGroupInstances(groupId string) ([]ec2.Reservation, error)
	ScanLoadBalancers() ([]elb.LoadBalancerDescription, error)
	ScanRDS() ([]rds.DBInstance, error)
	ScanRDSSecurityGroups() ([]rds.DBSecurityGroup, error)
}

type eC2ScannerImpl struct {
	credProvider 	*credentials.CredentialsProvider
}

func New(credProvider *credentials.CredentialsProvider) EC2Scanner {
	scanner := &eC2ScannerImpl{credProvider}
	return scanner
}

func (s *eC2ScannerImpl) getEC2Client() *ec2.EC2 {
	creds := s.credProvider.GetCredentials()
	awsCreds := aws.Creds(creds.AccessKeyId, creds.SecretAccessKey, "")
	return ec2.New(awsCreds, creds.Region, nil)
}

func (s *eC2ScannerImpl) getELBClient() *elb.ELB {
	creds := s.credProvider.GetCredentials()
	awsCreds := aws.Creds(creds.AccessKeyId, creds.SecretAccessKey, "")
	return elb.New(awsCreds, creds.Region, nil)
}

func (s *eC2ScannerImpl) getRDSClient() *rds.RDS {
	creds := s.credProvider.GetCredentials()
	awsCreds := aws.Creds(creds.AccessKeyId, creds.SecretAccessKey, "")
	return rds.New(awsCreds, creds.Region, nil)
}

func (s *eC2ScannerImpl) ScanSecurityGroups() ([]ec2.SecurityGroup, error) {
	client := s.getEC2Client()
	resp, err := client.DescribeSecurityGroups(nil)
	if err != nil {
		return nil, err
	}
	return resp.SecurityGroups, nil
}

func (s *eC2ScannerImpl) ScanSecurityGroupInstances(groupId string) ([]ec2.Reservation, error) {
	client := s.getEC2Client()
	resp, err := client.DescribeInstances(&ec2.DescribeInstancesRequest{
		Filters : []ec2.Filter{ec2.Filter{aws.String("instance.group-id"), []string{groupId}}}})
	if err != nil {
		return nil, err
	}
	return resp.Reservations, nil
}

func (s *eC2ScannerImpl) ScanLoadBalancers() ([]elb.LoadBalancerDescription, error) {
	client := s.getELBClient()
	resp, err := client.DescribeLoadBalancers(nil)
	if err != nil {
		return nil, err
	}
	return resp.LoadBalancerDescriptions, nil
}

func (s *eC2ScannerImpl) ScanRDS() ([]rds.DBInstance, error) {
	client := s.getRDSClient()
	resp, err := client.DescribeDBInstances(nil)
	if err != nil {
		return nil, err
	}
	return resp.DBInstances, nil
}

func (s *eC2ScannerImpl) ScanRDSSecurityGroups() ([]rds.DBSecurityGroup, error) {
	client := s.getRDSClient()
	resp, err := client.DescribeDBSecurityGroups(nil)
	if err != nil {
		return nil, err
	}
	return resp.DBSecurityGroups, nil
}