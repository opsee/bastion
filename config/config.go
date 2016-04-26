// The config package initializes global bastion configuration and provides a simple interface for interacting with
// that configuration data. It sets reasonable defaults that allow a build to pass.
package config

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var (
	config *Config = nil
)

func init() {
	viper.AutomaticEnv()

	// These defaults are optimized for simpler build/test cycles.
	// Customer/bastion info is from opsee+testing@opsee.com bastion testing user.
	// {
	// 	"id": 212,
	// 	"customer_id": "e4be4868-0b7c-11e6-8851-b7cb8c4dd4f0",
	// 	"email": "opsee+testing@opsee.com",
	// 	"name": "Opsee Testing",
	// 	"verified": true,
	// 	"active": true,
	// 	"created_at": 1461654172289,
	// 	"updated_at": 1461654172289
	// }
	viper.SetDefault("customer_id", "e4be4868-0b7c-11e6-8851-b7cb8c4dd4f0")
	viper.SetDefault("customer_email", "opsee+testing@opsee.com")
	viper.SetDefault("bartnet_host", "http://localhost:8080")
	viper.SetDefault("bastion_id", "8b90a924-0b7d-11e6-9c9f-b3cd43eb38df")
	viper.SetDefault("etcd_host", "etcd:2379")
	viper.SetDefault("nsqd_host", "nsqd:4150")
	viper.SetDefault("slate_host", "slate:7000")
	viper.SetDefault("log_level", "info")

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		config = NewConfig()
	})
}

// Global config provides shared aws session, metadata, and environmental variables declared in etc/opsee/bastion-env.sh.
type Config struct {
	CustomerId    string
	CustomerEmail string
	BartnetHost   string
	BastionId     string
	NsqdHost      string
	EtcdHost      string
	SlateHost     string
	LogLevel      string
	BezosHost     string
	AWS           *AWSConfig
}

func (this *Config) getAWSConfig() {
	awsConfig, err := NewAWSConfig()
	if err != nil {
		log.WithError(err).Fatal("Coudn't get AWS config.")
	} else {
		this.AWS = awsConfig
	}
}

func NewConfig() *Config {
	cfg := &Config{}

	level, err := log.ParseLevel(viper.GetString("log_level"))
	if err != nil {
		log.WithError(err).Warnf("Couldn't parse log level.")
	} else {
		log.SetLevel(level)
	}

	cfg.CustomerId = viper.GetString("customer_id")
	cfg.CustomerEmail = viper.GetString("customer_email")
	cfg.BastionId = viper.GetString("id")
	cfg.SlateHost = viper.GetString("slate_host")
	cfg.BartnetHost = viper.GetString("bartnet_host")
	cfg.NsqdHost = viper.GetString("nsqd_host")
	cfg.EtcdHost = viper.GetString("etcd_host")
	cfg.getAWSConfig()

	return cfg
}

// GetConfig returns a configuration object.
func GetConfig() *Config {
	if config == nil {
		config = NewConfig()
	}

	return config
}
