package main

import (
	"os"
	"os/signal"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/opsee/awscan"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/bastion/logging"
	"github.com/opsee/bastion/messaging"
)

const (
	moduleName = "discovery"
)

var (
	logger   = logging.GetLogger(moduleName)
	producer messaging.Producer
)

func main() {
	var err error
	cfg := config.GetConfig()
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
		MaxRetries:  aws.Int(11),
	})

	disco := awscan.NewDiscoverer(
		awscan.NewScanner(
			sess,
			cfg.MetaData.VPCID,
		),
	)

	producer, err = messaging.NewCustomerProducer(cfg.CustomerId, "discovery")

	if err != nil {
		panic(err)
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		log.Fatal(err.Error())
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, os.Kill)

	for {
		for event := range disco.Discover() {
			if event.Err != nil {
				logger.Error(event.Err.Error())
			} else {
				err = producer.Publish(event.Result)
				if err != nil {
					logger.Error(err.Error())
				}
			}
		}
		select {
		case s := <-sigs:
			log.Info("Received signal %s.  Stopping...", s)
			os.Exit(0)
		case err := <-heart.Beat():
			log.Error(err.Error())
		}
		time.Sleep(120 * time.Second)
	}
}
