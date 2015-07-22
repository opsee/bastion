package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/opsee/bastion/logging"
)

var (
	logger         = logging.GetLogger("config")
	config *Config = nil
)

type Config struct {
	AccessKeyId string // AWS Access Key Id
	SecretKey   string // AWS Secret Key
	Opsee       string // Opsee home IP address and port
	MDFile      string // Path to a file which overrides the instance meta
	CaPath      string // path to CA
	CertPath    string // path to TLS cert
	KeyPath     string // path to cert privkey
	DataPath    string // path to event logfile for replay
	CustomerId  string // The Customer ID
	AdminPort   uint   // Port for admin server.
	LogLevel    string // the log level to use
}

func GetConfig() *Config {
	if config == nil {
		config = &Config{}
		flag.StringVar(&config.AccessKeyId, "access_key_id", "", "AWS access key ID.")
		flag.StringVar(&config.SecretKey, "secret_key", "", "AWS secret key ID.")
		flag.StringVar(&config.Opsee, "opsee", "localhost:4080", "Hostname and port to the Opsee server.")
		flag.StringVar(&config.CaPath, "ca", "ca.pem", "Path to the CA certificate.")
		flag.StringVar(&config.CertPath, "cert", "cert.pem", "Path to the certificate.")
		flag.StringVar(&config.KeyPath, "key", "key.pem", "Path to the key file.")
		flag.StringVar(&config.DataPath, "data", "", "Data path.")
		flag.StringVar(&config.MDFile, "metadata", "", "Metadata path.")
		flag.StringVar(&config.CustomerId, "customer_id", "unknown-customer", "Customer ID.")
		flag.UintVar(&config.AdminPort, "admin_port", 4000, "Port for the admin server.")
		flag.StringVar(&config.LogLevel, "level", "info", "The log level to use")
		flag.Parse()
		err := logging.SetLevel(config.LogLevel, "bastion")
		if err != nil {
			fmt.Printf("%s is not a valid log level")
			os.Exit(1)
		}
	}
	return config
}
