package main

import (
	"os"
	"os/signal"
	"syscall"
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
	"github.com/opsee/bastion/messaging"
)

const (
	moduleName      = "discovery"
	discoveryPeriod = time.Second * 120
)

var (
	producer       messaging.Producer
	signalsChannel = make(chan os.Signal, 1)
)

func init() {
	signal.Notify(signalsChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

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
			cfg.MetaData.VpcId,
		),
	)

	producer, err = messaging.NewCustomerProducer(cfg.CustomerId, "discovery")

	if err != nil {
		panic(err)
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		log.WithError(err).Fatal("Couldn't initialize heartbeat!")
	}
	beatChan := heart.Beat()

	// Do discovery
	go func() {
		for {
			for event := range disco.Discover() {
				if event.Err != nil {
					log.WithError(err).Error("Event error during discovery.")
				} else {
					err = producer.Publish(event.Result)
					if err != nil {
						log.WithError(err).Error("Error publishing event during discovery.")
					}
				}
			}
			time.Sleep(discoveryPeriod)
		}
	}()

	for {
		select {
		case s := <-signalsChannel:
			switch s {
			case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				log.Info("Received signal ", s, ". Stopping.")
				os.Exit(0)
			}
		case beatErr := <-beatChan:
			log.WithError(beatErr)
		}
	}
}
