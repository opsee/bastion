package main

import (
	"encoding/json"
	"flag"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/op/go-logging"
	"github.com/opsee/bastion/aws"
	"github.com/opsee/bastion/netutil"
	"io/ioutil"
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
var (
	accessKeyId string // AWS Access Key Id
	secretKey   string // AWS Secret Key
	region      string // AWS Region Name
	opsee       string // Opsee home IP address
	caPath      string // path to CA
	certPath    string // path to TLS cert
	keyPath     string // path to cert privkey
	dataPath    string // path to event logfile for replay
	hostname    string // this machine's hostname
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	logging.SetLevel(logging.DEBUG, "bastion.main")
	logging.SetFormatter(logFormat)

	// cmdline args
	flag.StringVar(&accessKeyId, "access_key_id", "", "AWS access key ID.")
	flag.StringVar(&secretKey, "secret_key", "", "AWS secret key ID.")
	flag.StringVar(&region, "region", "", "AWS Region.")
	flag.StringVar(&opsee, "opsee", "localhost:4080", "Hostname and port to the Opsee server.")
	flag.StringVar(&caPath, "ca", "ca.pem", "Path to the CA certificate.")
	flag.StringVar(&certPath, "cert", "cert.pem", "Path to the certificate.")
	flag.StringVar(&keyPath, "key", "key.pem", "Path to the key file.")
	flag.StringVar(&dataPath, "data", "", "Data path.")
	flag.StringVar(&hostname, "hostname", "", "Hostname override.")
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
	awsScanner = aws.NewAwsApiEventParser(hostname, accessKeyId, secretKey, region)
	awsScanner.ConnectToOpsee(opsee)
	if dataPath != "" {
		go startStatic()
	} else {
		go start()
	}
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
	if events, err := loadEventsFromFile(dataPath); err != nil {
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

func loadEventsFromFile(dataFilePath string) (events []*netutil.Event, err error) {
	var file *os.File
	var bytes []byte

	const sendTickInterval = time.Second * 5

	if file, err = os.Open(dataFilePath); err != nil {
		log.Fatalf("opening data file %s: %v", dataPath, err)
	}
	if bytes, err = ioutil.ReadAll(file); err != nil {
		log.Fatalf("reading from data file %s: %v", dataPath, err)
	}
	if err = json.Unmarshal(bytes, &events); err != nil {
		log.Fatalf("unmarshalling json from %s: %v", dataPath, err)
	}
	return
}
