package ec2

import(
		"fmt"
		"github.com/stripe/aws-go/aws"
		"github.com/stripe/aws-go/gen/ec2"
		"bastion/credentials"
		// "github.com/stripe/aws-go/gen/autoscaling"
)

func Start(credProvider *credentials.CredentialsProvider) chan bool {
	c := make(chan bool)
	go func() {
		groups := make(map[string]ec2.SecurityGroup)
		instances := make(map[string]ec2.Instance)
		for {
			creds := credProvider.GetCredentials()
			fmt.Println("credentials:", creds)
			loop(creds, groups, instances)
			c <- true
		}
	}()
	return c
}

func loop(creds *credentials.Credentials, groups map[string]ec2.SecurityGroup, instances map[string]ec2.Instance) {
	awsCreds := aws.Creds(creds.AccessKeyId, creds.SecretAccessKey, "")
	ec2 := ec2.New(awsCreds, creds.Region, nil)
	iterateSecurityGroups(ec2, groups)
	fmt.Println("groups", groups)
	iterateInstances(ec2, instances)
}

func iterateSecurityGroups(client *ec2.EC2, groups map[string]ec2.SecurityGroup) {
	resp, err := client.DescribeSecurityGroups(nil)
	if err != nil {
		fmt.Println("encountered an error scanning the ec2 security groups API:", err)
		return
	}
	for _,group := range resp.SecurityGroups {
		fmt.Println("group", *group.GroupID)
		groups[*group.GroupID] = group
	}
}

func iterateInstances(client *ec2.EC2, instances map[string]ec2.Instance) {
	var token *string = nil
	resp, err := client.DescribeInstances(&ec2.DescribeInstancesRequest{NextToken:token})
	if err != nil {
		fmt.Println("encountered an error scanning ec2 instances API:", err)
		return
	}
	for _,reservation := range resp.Reservations {
		for _,instance := range reservation.Instances {
			instances[*instance.InstanceID] = instance
		}
	}
}