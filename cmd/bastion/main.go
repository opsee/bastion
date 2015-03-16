package main

import (
	"bastion/credentials"
	"bastion/netutil"
	"bastion/scanner"
	"flag"
	"fmt"
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
	"strconv"
	"strings"
)

var (
	log       = logging.MustGetLogger("bastion.json-tcp")
	logFormat = logging.MustStringFormatter("%{time:2006-01-02T15:04:05.999999999Z07:00} %{level} [%{module}] %{message}")
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
	logging.SetLevel(logging.INFO, "json-tcp")
	logging.SetFormatter(logFormat)
	// cmdline args
	flag.StringVar(&accessKeyId, "access_key_id", "", "AWS access key ID.")
	flag.StringVar(&secretKey, "secret_key", "", "AWS secret key ID.")
	flag.StringVar(&region, "region", "", "AWS Region.")
	flag.StringVar(&opsee, "opsee", "localhost:8085", "Hostname and port to the Opsee server.")
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
    isShutdown  := request.Command == "shutdown"
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


var (
    jsonServer netutil.TCPServer
    ec2Client scanner.EC2Scanner
    opseeClient *raidman.Client
)

func loadAndPopulate() (err error) {
    if dataPath != "" {
        file, err := os.Open(dataPath)
        if err != nil {
            panic(err)
        }
        bytes, err := ioutil.ReadAll(file)
        if err != nil {
            panic(err)
        }
        events := []raidman.Event{}
        err = json.Unmarshal(bytes, &events)
        if err != nil {
            panic(err)
        }
        discTick := time.Tick(time.Second * 5)
        for _, event := range events {
            <-discTick
            fmt.Println(event)
            opseeClient.Send(&event)
        }
    } else {
        var groups []ec2.SecurityGroup
        if groups, err = ec2Client.ScanSecurityGroups(); err != nil {
            log.Fatalf("scanning security groups: %s", err.Error())
        }

        groupMap := make(map[string]ec2.SecurityGroup)
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
            event := raidman.Event{}
            event.Ttl = 120
            event.Host = hostname
            event.Service = "discovery"
            event.State = "sg"
            event.Metric = 0
            event.Attributes = make(map[string]string)
            event.Attributes["group_id"] = *group.GroupID
            if group.GroupName != nil {
                event.Attributes["group_name"] = *group.GroupName
            }
            if len(group.IPPermissions) > 0 {
                perms := group.IPPermissions[0]
                if perms.ToPort != nil {
                    event.Attributes["port"] = strconv.Itoa(*perms.ToPort)
                }
                if perms.IPProtocol != nil {
                    event.Attributes["protocol"] = *perms.IPProtocol
                }
            }
            fmt.Println(event)
            opseeClient.Send(&event)
        }
        lbs, _ := ec2Client.ScanLoadBalancers()
        for _, lb := range lbs {
            if lb.LoadBalancerName == nil {
                continue
            }
            event := raidman.Event{}
            event.Ttl = 120
            event.Host = hostname
            event.Service = "discovery"
            event.State = "elb"
            event.Metric = 0
            event.Attributes = make(map[string]string)
            event.Attributes["group_name"] = *lb.LoadBalancerName
            event.Attributes["group_id"] = *lb.DNSName
            if lb.HealthCheck != nil {
                split := strings.Split(*lb.HealthCheck.Target, ":")
                split2 := strings.Split(split[1], "/")
                event.Attributes["port"] = split2[0]
                event.Attributes["protocol"] = split[0]
                event.Attributes["request"] = strings.Join([]string{"/", split2[1]}, "")
            }
            fmt.Println(event)
            opseeClient.Send(&event)
        }
        rdbs, _ := ec2Client.ScanRDS()
        sgs, _ := ec2Client.ScanRDSSecurityGroups()
        sgMap := make(map[string]rds.DBSecurityGroup)
        for _, sg := range sgs {
            if sg.DBSecurityGroupName != nil {
                sgMap[*sg.DBSecurityGroupName] = sg
            }
        }
        for _, db := range rdbs {
            event := raidman.Event{}
            event.Ttl = 120
            event.Host = hostname
            event.Service = "discovery"
            event.State = "rds"
            event.Metric = 0
            event.Attributes = make(map[string]string)
            if db.DBName != nil {
                event.Attributes["group_name"] = *db.DBName
                if len(db.VPCSecurityGroups) > 0 {
                    sgId := *db.VPCSecurityGroups[0].VPCSecurityGroupID
                    // sg := sgMap[sgId]
                    event.Attributes["group_id"] = sgId
                    ec2sg := groupMap[sgId]
                    perms := ec2sg.IPPermissions[0]
                    event.Attributes["port"] = strconv.Itoa(*perms.ToPort)
                    event.Attributes["protocol"] = "sql"
                    event.Attributes["request"] = "select 1;"
                }
            }
            fmt.Println(event)
            opseeClient.Send(&event)
        }
        //FIN
        event := raidman.Event{}
        event.Ttl = 120
        event.Host = hostname
        event.Service = "discovery"
        event.State = "end"
        fmt.Println(event)
        opseeClient.Send(&event)
    }
    return
}

func main() {
    defer func() {
        jsonServer.Close()
        jsonServer.Join()
    }()
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()
    var err error
	if jsonServer, err = netutil.ListenTCP(":5666", &Server{}); err != nil {
		log.Panic("json-tcp server failed to start: ", err)
		return
	}
	httpClient := &http.Client{}
	credProvider := credentials.NewProvider(httpClient, accessKeyId, secretKey, region)
	ec2Client = scanner.New(credProvider)
	opseeClient, err := raidman.Dial("tcp", opsee)
	if err != nil { //we'll need retry logic here but for right now I just need the frickin build to go
		fmt.Println("err", err)
		time.Sleep(30 * time.Second)
        return
	}

	if hostname == "" {
		hostname = credProvider.GetInstanceId().InstanceId
	}

	tick := time.Tick(time.Second * 10)

	go loadAndPopulate()

	for {
		event := &raidman.Event{
			State:   "connected",
			Host:    hostname,
			Service: "bastion",
			Ttl:     10}
		fmt.Println(event)
		opseeClient.Send(event)
		<-tick
	}
}
