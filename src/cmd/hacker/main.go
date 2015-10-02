package main

import (
	"io/ioutil"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/opsee/awscan"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/logging"
)

func main() {
	logger := logging.GetLogger("hacker")
	cfg := config.GetConfig()

	sc := awscan.NewScanner(&awscan.Config{AccessKeyId: cfg.AccessKeyId, SecretKey: cfg.SecretKey, Region: cfg.MetaData.Region})

	httpClient := &http.Client{}
	metap := config.NewMetadataProvider(httpClient, cfg)
	metadata := metap.Get()
	region := metadata.Region

	var creds = credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.StaticProvider{Value: credentials.Value{
				AccessKeyID:     cfg.AccessKeyId,
				SecretAccessKey: cfg.SecretKey,
				SessionToken:    "",
			}},
			&credentials.EnvProvider{},
			&ec2rolecreds.EC2RoleProvider{ExpiryWindow: 5 * time.Minute},
		})

	awsConfig := &aws.Config{Credentials: creds, Region: aws.String(region)}

	ec2Client := ec2.New(awsConfig)

	resp, err := httpClient.Get("http://169.254.169.254/latest/meta-data/security-groups/")
	if err != nil {
		logger.Error(err.Error())
		logger.Fatal("Unable to get security group from meta data service")
	}

	defer resp.Body.Close()
	secGroupName, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Fatal(err.Error())
	}

	output, err := ec2Client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name: aws.String("group-name"),
				Values: []*string{
					aws.String(string(secGroupName)),
				},
			},
		},
	})

	if len(output.SecurityGroups) != 1 {
		logger.Fatal("Bad number of bastion security groups found: %d", len(output.SecurityGroups))
	}

	bastionSgId := output.SecurityGroups[0].GroupId

	var found bool

	for {
		sgs, err := sc.ScanSecurityGroups()
		if err != nil {
			logger.Error(err.Error())
		}

		for _, sg := range sgs {
			logger.Debug("Found security group: %v", *sg)
			found = false
			for _, perm := range sg.IpPermissions {
				for _, ipr := range perm.IpRanges {
					if ipr.CidrIp == bastionSgId {
						found = true
					}
				}
			}

			if !found {
				_, err := ec2Client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
					GroupId: sg.GroupId,
					IpPermissions: []*ec2.IpPermission{
						&ec2.IpPermission{
							IpProtocol: aws.String("-1"),
							FromPort:   aws.Int64(0),
							ToPort:     aws.Int64(65535),
							UserIdGroupPairs: []*ec2.UserIdGroupPair{
								&ec2.UserIdGroupPair{
									GroupId: bastionSgId,
								},
							},
						},
					},
				})
				if err != nil {
					logger.Error("Unable to add ourselves to security group: %s", *sg.GroupId)
					logger.Error(err.Error())
				} else {
					logger.Info("Added ourselves to security group: %s", *sg.GroupId)
				}
			}
			// Janky, but in order to space out our requests, slow us down some.
			time.Sleep(1 * time.Second)
		}

		// Only run this every half hour.
		time.Sleep(30 * time.Minute)
	}
}
