package main

import (
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/opsee/awscan"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
)

var (
	moduleName    = "hacker"
	unhackFlagPtr = flag.Bool("unhack", false, "detach from security groups")
	sgidFlagStr   = flag.String("sgid", "none", "id of the security group")
	bastionSgId   = aws.String("none")
	sigs          = make(chan os.Signal, 1)
	httpClient    = &http.Client{}
	ec2Client     = &ec2.EC2{}
	heartbeat     = &heart.Heart{}
	sc            awscan.EC2Scanner
)

func main() {
	signal.Notify(sigs, os.Interrupt, os.Kill)

	cfg := config.GetConfig()
	sc = awscan.NewScanner(&awscan.Config{AccessKeyId: cfg.AccessKeyId, SecretKey: cfg.SecretKey, Region: cfg.MetaData.Region})

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

	awsConfig := &aws.Config{Credentials: creds, Region: aws.String(cfg.MetaData.Region)}
	ec2Client = ec2.New(awsConfig)

	var err error
	heartbeat, err = heart.NewHeart(moduleName)
	if err != nil {
		log.Fatal(err.Error())
	}

	// loop until sigkill
	for {
		// get the bastion's security group
		aws.String(*sgidFlagStr)

		// if no security group was passed in, get one
		if *bastionSgId == "none" {
			resp, err := httpClient.Get("http://169.254.169.254/latest/meta-data/security-groups/")
			if err != nil {
				log.Error(err.Error())
				log.Fatal("Unable to get security group from meta data service")
			}

			defer resp.Body.Close()
			secGroupName, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err.Error())
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
				log.Fatal("Bad number of bastion security groups found: %d", len(output.SecurityGroups))
			}

			bastionSgId = output.SecurityGroups[0].GroupId
		}

		log.Info(*bastionSgId)

		// either hack or unhack
		hack(*unhackFlagPtr, *bastionSgId)

		if *unhackFlagPtr {
			break
		}

		// hack to get the signal and run unhack
		select {
		case s := <-sigs:
			log.Info("Received signal %s.  Stopping...", s)
			os.Exit(0)
		case <-time.After(25 * time.Minute):
			continue
		}
	}
}

func hack(unhack bool, sgid string) {
	var found bool

	ippermission := []*ec2.IpPermission{
		&ec2.IpPermission{
			IpProtocol: aws.String("-1"),
			FromPort:   aws.Int64(0),
			ToPort:     aws.Int64(65535),
			UserIdGroupPairs: []*ec2.UserIdGroupPair{
				&ec2.UserIdGroupPair{
					GroupId: bastionSgId,
				},
			},
		}}

	sgs, err := sc.ScanSecurityGroups()

	if err != nil {
		log.Error(err.Error())
	}

	for _, sg := range sgs {
		found = false
		for _, perm := range sg.IpPermissions {
			for _, ipr := range perm.IpRanges {
				if ipr.CidrIp == bastionSgId {
					found = true
				}
			}
		}

		// Add ourselves to the security group, we're not in it yet and it's not a bastion
		if !unhack && !found {
			_, err := ec2Client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
				GroupId:       sg.GroupId,
				IpPermissions: ippermission,
			})
			if err != nil {
				log.Error("Unable to add ourselves to security group: ", *sg.GroupId)
				log.Error(err.Error())
			} else {
				log.Info("Added ourselves to security group: ", *sg.GroupId)
			}
		}

		// remove ourselves from the security group
		if unhack {
			_, err := ec2Client.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
				GroupId:       sg.GroupId,
				IpPermissions: ippermission,
			})
			if err != nil {
				log.Error("Unable to remove ourselves to security group: ", *sg.GroupId)
				log.Error(err.Error())
			} else {
				log.Info("Removed ourselves from security group: ", *sg.GroupId)
			}
		}

		select {
		case s := <-sigs:
			log.Info("Received signal %s.  Stopping...", s)
			os.Exit(0)
		case <-time.After(500 * time.Millisecond):
			continue
		}
	}
}
