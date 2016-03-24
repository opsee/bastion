package config

import (
	"os"
	"sync"

	log "github.com/Sirupsen/logrus"
)

var (
	config *Config = nil
	once   sync.Once
)

// Global config provides shared aws session, metadata, and environmental variables declared in etc/opsee/bastion-env.sh.
type Config struct {
	CustomerId          string
	CustomerEmail       string
	BastionAuthEndpoint string
	BartnetHost         string
	BastionAuthType     string
	BastionId           string
	NsqdHost            string
	EtcdHost            string
	SlateHost           string
	LogLevel            string
	AWS                 *AWSConfig
}

func (this *Config) getAWSConfig() {
	awsConfig, err := NewAWSConfig()
	if err != nil {
		log.WithError(err).Fatal("Coudn't get AWS config.")
	} else {
		this.AWS = awsConfig
	}
}

func (this *Config) setLogLevel(defaultLevel log.Level) {
	level, err := log.ParseLevel(this.LogLevel)
	if err != nil {
		log.WithError(err).Warnf("Couldn't parse log level.  Using default level %s.", defaultLevel)
		log.SetLevel(defaultLevel)
	} else {
		log.Infof("Setting log level to %s", this.LogLevel)
		log.SetLevel(level)
	}
}

func (this *Config) getEnv() {
	this.LogLevel = os.Getenv("LOG_LEVEL")
	this.CustomerId = os.Getenv("CUSTOMER_ID")
	this.CustomerEmail = os.Getenv("CUSTOMER_EMAIL")
	this.BastionId = os.Getenv("BASTION_ID")
	this.SlateHost = os.Getenv("SLATE_HOST")
	this.BartnetHost = os.Getenv("BARTNET_HOST")
	this.BastionAuthType = os.Getenv("BASTION_AUTH_TYPE")
	this.BastionAuthEndpoint = os.Getenv("BASTION_AUTH_ENDPOINT")
	this.NsqdHost = os.Getenv("NSQD_HOST")
	this.EtcdHost = os.Getenv("ETCD_HOST")
}

func GetConfig() *Config {
	once.Do(func() {
		config = &Config{}
		config.getEnv()
		config.setLogLevel(log.ErrorLevel)
		config.getAWSConfig()
	})

	return config
}
