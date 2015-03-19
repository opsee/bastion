package main

import (
	"bastion/credentials"
	"bastion/netutil"
	"bastion/scanner"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"time"
	// "bastion/resilient"
	"encoding/json"
	"github.com/amir/raidman"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
	"github.com/awslabs/aws-sdk-go/gen/rds"
	"github.com/op/go-logging"
	"runtime"
)

var (
	log       = logging.MustGetLogger("main")
	logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} [%{level:.8s}]: [%{module}] %{shortfunc} ▶ %{id:03x}%{color:reset} %{message}")
)

// we must first retrieve our AWS API keys, which will either be in the instance metadata,
// or our command line options. Then we begin scanning the environment, first using the AWS
// API, and then actually trying to open TCP connections.

// In parallel we try and open a TLS connection back to the opsee API. We'll have been supplied
// a ca certificate, certificate and a secret key in pem format, either via the instance metadata
// or on the command line.
var (
	accessKeyId string
	secretKey   string
	region      string
	opsee       string
	caPath      string
	certPath    string
	keyPath     string
	dataPath    string
	hostname    string
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	logging.SetLevel(logging.DEBUG, "bastion.main")
	logging.SetFormatter(logFormat)

	// cmdline args
	flag.StringVar(&accessKeyId, "access_key_id", "", "AWS access key ID.")
	flag.StringVar(&secretKey, "secret_key", "", "AWS secret key ID.")
	flag.StringVar(&region, "region", "", "AWS Region.")
	flag.StringVar(&opsee, "opsee", "localhost:5556", "Hostname and port to the Opsee server.")
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

func (this *Server) RequestReceived(connection *netutil.Connection, request *netutil.ServerRequest) (reply *netutil.Reply, keepGoing bool) {
	isShutdown := request.Command == "shutdown"
	keepGoing = !isShutdown
	if isShutdown {
		if err := connection.Server().Close(); err != nil {
			log.Notice("shutdown")
			reply = nil
		}
	}
	reply = netutil.NewReply(request)
	return
}

type OpseeClient struct {
	*raidman.Client
}

func NewOpseeClient(address string) (client *raidman.Client, err error) {
	return raidman.Dial("tcp", address)
}

var (
	httpClient        *http.Client = &http.Client{}
	credProvider      *credentials.CredentialsProvider
	jsonServer        netutil.TCPServer
	ec2Client         scanner.EC2Scanner
	awsApiEventParser *scanner.AwsApiEventParser
	opseeClient       *raidman.Client              = nil
	groupMap          map[string]ec2.SecurityGroup = make(map[string]ec2.SecurityGroup)
)

func main() {
	var err error

	flag.Parse()

	if jsonServer, err = netutil.ListenTCP(":5666", &Server{}); err != nil {
		log.Fatalf("json-tcp server failed to start: ", err)
	}

	httpClient = &http.Client{}
	credProvider = credentials.NewProvider(httpClient, accessKeyId, secretKey, region)
	if hostname == "" {
		if credProvider.GetInstanceId() == nil {
			log.Fatalf("Cannot determine hostname")
		} else {
			hostname = credProvider.GetInstanceId().InstanceId
		}
	}
	log.Info("hostname: %s", hostname)

	awsApiEventParser = scanner.NewAwsApiEventParser(hostname)
	ec2Client = scanner.New(credProvider)

	connectToOpsee := func() (interface{}, error) { return NewOpseeClient(opsee) }
	connectToOpseeRetrier := netutil.NewBackoffRetrier(connectToOpsee)
	if err = connectToOpseeRetrier.Run(); err != nil {
		log.Fatalf("connectToOpsee: %v", err)
	}
	opseeClient = connectToOpseeRetrier.Result().(*raidman.Client)

	go loadAndPopulate()
	go connectionIdleLoop()

	jsonServer.Join()
}

func connectionIdleLoop() {
	tick := time.Tick(time.Second * 10)
	connectedEvent := awsApiEventParser.NewEvent("bastion")
	connectedEvent.State = "connected"
	connectedEvent.Ttl = 10
	for {
		log.Debug("%+v", connectedEvent)
		opseeClient.Send(connectedEvent)
		<-tick
	}
}

func reportFromDataFile(dataFilePath string) (err error) {
	var events []raidman.Event
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
	discTick := time.Tick(sendTickInterval)
	for _, event := range events {
		<-discTick
		log.Debug("%+v", event)
		opseeClient.Send(&event)
	}
	return
}

func loadAndPopulate() (err error) {
	if dataPath != "" {
		return reportFromDataFile(dataPath)
	}
	var groups []ec2.SecurityGroup
	if groups, err = ec2Client.ScanSecurityGroups(); err != nil {
		log.Error("scanning security groups: %s", err.Error())
		return
	}

	for _, group := range groups {
		if group.GroupID != nil {
			groupMap[*group.GroupID] = group
			instances, _ := ec2Client.ScanSecurityGroupInstances(*group.GroupID)
			if len(instances) == 0 {
				continue
			}
		} else {
			continue
		}
		event := awsApiEventParser.ToEvent(group)
		log.Debug("%+v", event)
		opseeClient.Send(event)
	}

	lbs, _ := ec2Client.ScanLoadBalancers()
	for _, lb := range lbs {
		if lb.LoadBalancerName != nil {
			event := awsApiEventParser.ToEvent(lb)
			log.Debug("%+v", event)
			opseeClient.Send(event)
		}
	}

	sgs, _ := ec2Client.ScanRDSSecurityGroups()
	sgMap := make(map[string]rds.DBSecurityGroup)
	for _, sg := range sgs {
		if sg.DBSecurityGroupName != nil {
			sgMap[*sg.DBSecurityGroupName] = sg
		}
	}

	rdbs, _ := ec2Client.ScanRDS()
	for _, db := range rdbs {
		event := awsApiEventParser.ToEvent(db)
		log.Debug("%+v", event)
		opseeClient.Send(event)
	}

	//FIN
	event := awsApiEventParser.NewEvent("discovery")
	event.State = "end"
	log.Debug("%+v", event)
	opseeClient.Send(event)
	return
}
