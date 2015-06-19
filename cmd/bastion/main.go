package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/op/go-logging"
	"github.com/opsee/bastion/aws"
	"github.com/opsee/bastion/netutil"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"time"
)

var (
	log       = logging.MustGetLogger("bastion")
	logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} [%{level:.8s}]: [%{module}] %{shortfunc} ▶ %{id:03x}%{color:reset} %{message}")
)

const sendTickInterval = time.Second * 5

// We must first retrieve our AWS API keys, which will either be in the inthotance metadata,
// or our command line options. Then we begin scanning the environment, first using the AWS
// API, and then actually trying to open TCP connections.

// In parallel we try and open a TLS connection back to the opsee API. We'll have been supplied
// a ca certificate, certificate and a secret key in pem format, either via the instance metadata
// or on the command line.
type BastionConfig struct {
	AccessKeyId string // AWS Access Key Id
	SecretKey   string // AWS Secret Key
	Region      string // AWS Region Name
	Opsee       string // Opsee home IP address
	CaPath      string // path to CA
	CertPath    string // path to TLS cert
	KeyPath     string // path to cert privkey
	DataPath    string // path to event logfile for replay
	Hostname    string // this machine's hostname
	CustomerId  string // the customer ID we're connecting under
	AdminPort   uint   // the admin port number to listen on
}

var (
	config BastionConfig
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	logging.SetLevel(logging.DEBUG, "bastion.main")
	logging.SetFormatter(logFormat)

	// cmdline args
	flag.StringVar(&config.AccessKeyId, "access_key_id", "", "AWS access key ID.")
	flag.StringVar(&config.SecretKey, "secret_key", "", "AWS secret key ID.")
	flag.StringVar(&config.Region, "region", "", "AWS Region.")
	flag.StringVar(&config.Opsee, "opsee", "localhost:4080", "Hostname and port to the Opsee server.")
	flag.StringVar(&config.CaPath, "ca", "ca.pem", "Path to the CA certificate.")
	flag.StringVar(&config.CertPath, "cert", "cert.pem", "Path to the certificate.")
	flag.StringVar(&config.KeyPath, "key", "key.pem", "Path to the key file.")
	flag.StringVar(&config.DataPath, "data", "", "Data path.")
	flag.StringVar(&config.Hostname, "hostname", "", "Hostname override.")
	flag.StringVar(&config.CustomerId, "customer_id", "unknown-customer", "Customer ID.")
	flag.UintVar(&config.AdminPort, "admin_port", 4000, "Port for the admin server.")
}

type Server struct{}

func (this *Server) SslOptions() netutil.SslOptions {
	return nil
}

func (this *Server) ConnectionMade(connection *netutil.Connection) bool {
	return true
}

func (this *Server) ConnectionLost(connection *netutil.Connection, err error) {
	log.Error("Connection lost: %v", err)
}

func (this *Server) RequestReceived(connection *netutil.Connection, request interface{}) (reply interface{}, keepGoing bool) {
	log.Info("Received a request for %+v", request)
	return nil, true
}

func MustStartServer() (server netutil.TCPServer) {
	var err error
	if server, err = netutil.ListenTCP(":5666", &Server{}); err != nil {
		log.Fatalf("json-tcp server failed to start: ", err)
	}
	return
}

var awsScanner *aws.AwsApiEventParser

func main() {
	flag.Parse()
	awsScanner = aws.NewAwsApiEventParser(config.Hostname, config.AccessKeyId, config.SecretKey, config.Region, config.CustomerId)
	awsScanner.ConnectToOpsee(config.Opsee)
	if config.DataPath != "" {
		go startStatic()
	} else {
		go start()
	}
	go startHealthStatusServer()
	jsonServer := MustStartServer()
	jsonServer.Join()
}

func start() {
	if err := awsScanner.Scan(); err == nil {
		awsScanner.RunForever()
	} else {
		log.Fatal("Scan failed: %v", err)
	}
}

func startStatic() {
	if events, err := loadEventsFromFile(config.DataPath); err != nil {
		log.Fatal("loadEventsFromFile: %+v", events)
	} else {
		reportStaticEvents(events)
	}
}

func reportStaticEvents(events []*netutil.Event) {
	discTick := time.Tick(sendTickInterval)
	for _, event := range events {
		<-discTick
		eventMessage := awsScanner.MessageMaker.NewEventMessage()
		eventMessage.Event = *event
		awsScanner.SendEvent(eventMessage)
	}
}

func startHealthStatusServer() {
	http.HandleFunc("/health_status", func(w http.ResponseWriter, req *http.Request) {
		encoder := json.NewEncoder(w)
		encoder.Encode(config)
		return
	})
	log.Fatal(http.ListenAndServe(fmt.Sprint(":", config.AdminPort), nil))
}

func loadEventsFromFile(dataFilePath string) (events []*netutil.Event, err error) {
	var file *os.File
	var bytes []byte

	const sendTickInterval = time.Second * 5

	if file, err = os.Open(dataFilePath); err != nil {
		log.Fatalf("opening data file %s: %v", config.DataPath, err)
	}
	if bytes, err = ioutil.ReadAll(file); err != nil {
		log.Fatalf("reading from data file %s: %v", config.DataPath, err)
	}
	if err = json.Unmarshal(bytes, &events); err != nil {
		log.Fatalf("unmarshalling json from %s: %v", config.DataPath, err)
	}
	return
}
