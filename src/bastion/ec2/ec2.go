package ec2

import(
		"fmt"
		"github.com/stripe/aws-go/aws"
		"github.com/stripe/aws-go/gen/ec2"
		"bastion/credentials"
		// "github.com/stripe/aws-go/gen/autoscaling"
)

type EC2Scanner interface {
	ScanSecurityGroups() ([]*ec2.SecurityGroup, error)
	ScanSecurityGroupInstances() ([]*ec2.Reservation, error)
}

type eC2ScannerImpl struct {
	credProvider 	*credentials.CredentialsProvider
}

func New(credProvider *credentials.CredentialsProvider) EC2Scanner {
	scanner := &eC2ScannerImpl{credProvider}

}

func (s *eC2ScannerImpl) getClient() *ec2.EC2 {
	creds := s.credProvider.GetCredentials()
	awsCreds := aws.Creds(creds.AccessKeyId, creds.SecretAccessKey, "")
	return ec2.New(awsCreds, creds.Region, nil)
}

func (s *eC2ScannerImpl) ScanSecurityGroups() ([]*ec2.SecurityGroup, error) {
	client := s.getClient()
	resp, err := client.DescribeSecurityGroups(nil)
	if err != nil {
		return nil, err
	}
	return resp.SecurityGroups, nil
}

func (s *EC2Scanner) ScanSecurityGroupInstances(groupId string) ([]*ec2.Reservation, error) {
	client := s.getClient()
	resp, err := client.DescribeInstances(&ec2.DescribeInstancesRequest{
		Filters : []ec2.Filter{ec2.Filter{"SecurityGroups", []string{groupId}}}})
	if err != nil {
		return nil, err
	}
	return resp.Reservations, nil
}
