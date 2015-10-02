package config

import (
	"flag"
	"fmt"
	"github.com/opsee/bastion/logging"
	"net/http"
	"os"
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
	MetaData    *InstanceMeta
}

var (
	logger         = logging.GetLogger("config")
	config *Config = nil
)

func GetConfig() *Config {
	if config == nil {
		config = &Config{}

		flag.StringVar(&config.AccessKeyId, "access_key_id", os.Getenv("AWS_ACCESS_KEY_ID"), "AWS access key ID.")
		flag.StringVar(&config.SecretKey, "secret_key", os.Getenv("AWS_SECRET_ACCESS_KEY"), "AWS secret key ID.")
		flag.StringVar(&config.Opsee, "opsee", os.Getenv("BARTNET_HOST"), "Hostname and port to the Opsee server.")
		flag.StringVar(&config.CaPath, "ca", os.Getenv("CA_PATH"), "Path to the CA certificate.")
		flag.StringVar(&config.CertPath, "cert", os.Getenv("CERT_PATH"), "Path to the certificate.")
		flag.StringVar(&config.KeyPath, "key", os.Getenv("KEY_PATH"), "Path to the key file.")
		flag.StringVar(&config.CustomerId, "customer_id", os.Getenv("CUSTOMER_ID"), "Customer ID.")

		flag.StringVar(&config.DataPath, "data", "", "Data path.")
		flag.StringVar(&config.MDFile, "metadata", "", "Metadata path.")
		flag.UintVar(&config.AdminPort, "admin_port", 4000, "Port for the admin server.")
		flag.StringVar(&config.LogLevel, "level", "info", "The log level to use")
		flag.Parse()

		// get metadata (should be from file if file provided)
		httpClient := &http.Client{}
		metap := NewMetadataProvider(httpClient, config)
		config.MetaData = metap.Get()

		err := logging.SetLevel(config.LogLevel, "bastion")
		if err != nil {
			fmt.Printf("%s is not a valid log level")
			os.Exit(1)
		}
	}

	return config
}
