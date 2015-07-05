package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/rds"
)

type EC2Scanner interface {
	ScanSecurityGroups() ([]*ec2.SecurityGroup, error)
	ScanSecurityGroupInstances(groupId string) ([]*ec2.Reservation, error)
	ScanLoadBalancers() ([]*elb.LoadBalancerDescription, error)
	ScanRDS() ([]*rds.DBInstance, error)
	ScanRDSSecurityGroups() ([]*rds.DBSecurityGroup, error)
}

type eC2ScannerImpl struct {
	config *aws.Config
}

func NewScanner(config *aws.Config) EC2Scanner {
	scanner := &eC2ScannerImpl{config}
	return scanner
}

func (s *eC2ScannerImpl) getConfig() *aws.Config {
	return s.config
}

func (s *eC2ScannerImpl) getEC2Client() *ec2.EC2 {
	return ec2.New(s.getConfig())
}

func (s *eC2ScannerImpl) getELBClient() *elb.ELB {

	return elb.New(s.getConfig())
}

func (s *eC2ScannerImpl) getRDSClient() *rds.RDS {
	return rds.New(s.getConfig())
}

func (s *eC2ScannerImpl) ScanSecurityGroups() ([]*ec2.SecurityGroup, error) {
	client := s.getEC2Client()
	resp, err := client.DescribeSecurityGroups(nil)
	if err != nil {
		return nil, err
	}
	return resp.SecurityGroups, nil
}

func (s *eC2ScannerImpl) ScanSecurityGroupInstances(groupId string) ([]*ec2.Reservation, error) {
	client := s.getEC2Client()
	var grs []*string = []*string{&groupId}
	filters := []*ec2.Filter{&ec2.Filter{Name: aws.String("instance.group-id"), Values: grs}}
	//[]string{groupId}}}})
	resp, err := client.DescribeInstances(&ec2.DescribeInstancesInput{Filters: filters})
	if err != nil {
		return nil, err
	}
	return resp.Reservations, nil
}

func (s *eC2ScannerImpl) ScanLoadBalancers() ([]*elb.LoadBalancerDescription, error) {
	client := s.getELBClient()
	resp, err := client.DescribeLoadBalancers(nil)
	if err != nil {
		return nil, err
	}
	return resp.LoadBalancerDescriptions, nil
}

func (s *eC2ScannerImpl) ScanRDS() ([]*rds.DBInstance, error) {
	client := s.getRDSClient()
	resp, err := client.DescribeDBInstances(nil)
	if err != nil {
		return nil, err
	}
	return resp.DBInstances, nil
}

func (s *eC2ScannerImpl) ScanRDSSecurityGroups() ([]*rds.DBSecurityGroup, error) {
	client := s.getRDSClient()
	resp, err := client.DescribeDBSecurityGroups(nil)
	if err != nil {
		return nil, err
	}
	return resp.DBSecurityGroups, nil
}
