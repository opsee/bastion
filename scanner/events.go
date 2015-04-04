package scanner

import (
	"fmt"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/amir/raidman"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/service/elb"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/service/rds"
	"github.com/opsee/bastion/credentials"
	"github.com/opsee/bastion/netutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEventTtl = 120
)

func MustConnectToOpsee(address string) *raidman.Client {
	connectToOpsee := func() (interface{}, error) { return raidman.Dial("tcp", address) }
	connectToOpseeRetrier := netutil.NewBackoffRetrier(connectToOpsee)
	if err := connectToOpseeRetrier.Run(); err != nil {
		log.Fatalf("connectToOpsee: %v", err)
	}
	return connectToOpseeRetrier.Result().(*raidman.Client)
}

type AwsApiEventParser struct {
	Hostname     string
	accessKeyId  string
	secretKey    string
	region       string
	httpClient   *http.Client
	CredProvider *credentials.CredentialsProvider
	EC2Client    EC2Scanner
	GroupMap     map[string]ec2.SecurityGroup
	opseeClient  *raidman.Client
}

func NewAwsApiEventParser(hostname string, accessKeyId string, secretKey string, region string) *AwsApiEventParser {
	httpClient := &http.Client{}
	credProvider := credentials.NewProvider(httpClient, accessKeyId, secretKey, region)
	return &AwsApiEventParser{
		Hostname:     hostname,
		accessKeyId:  accessKeyId,
		secretKey:    secretKey,
		region:       region,
		httpClient:   httpClient,
		CredProvider: credProvider,
		EC2Client:    NewScanner(credProvider),
		GroupMap:     make(map[string]ec2.SecurityGroup),
	}
}

func (a *AwsApiEventParser) ConnectToOpsee(address string) {
	a.opseeClient = MustConnectToOpsee(address)
}

func (a *AwsApiEventParser) Scan() (err error) {
	defer a.FinishDiscovery()
	if err = a.ScanSecurityGroups(); err == nil {
		if err = a.ScanLoadBalancers(); err == nil {
			if err = a.ScanRDS(); err != nil {
				log.Error("AwsApiEventParser.ScanRDS: %v", err)
			}
		} else {
			log.Error("AwsApiEventParse.ScanLoadBalancers: %v", err)
		}
	} else {
		log.Error("AwsApiEventParser.ScanSecurityGroup: %v", err)
	}
	return
}

func (a *AwsApiEventParser) ScanSecurityGroups() (err error) {
	if groups, err := a.EC2Client.ScanSecurityGroups(); err != nil {
		log.Error("scanning security groups: %s", err.Error())
	} else {
		for _, group := range groups {
			if group.GroupID != nil {
				a.GroupMap[*group.GroupID] = *group
				instances, _ := a.EC2Client.ScanSecurityGroupInstances(*group.GroupID)
				if len(instances) == 0 {
					continue
				}
			} else {
				continue
			}
			err = a.SendEvent(a.ToEvent(group))
		}
	}
	return
}

func (a *AwsApiEventParser) ScanRDS() (err error) {
	if rdbs, err := a.EC2Client.ScanRDS(); err != nil {
		log.Error("ScanRDS: %v", err)
	} else {
		for _, db := range rdbs {
			err = a.SendEvent(a.ToEvent(db))
		}
	}
	return
}

func (a *AwsApiEventParser) ScanLoadBalancers() (err error) {
	if lbs, err := a.EC2Client.ScanLoadBalancers(); err == nil {
		for _, lb := range lbs {
			if lb.LoadBalancerName != nil {
				err = a.opseeClient.Send(a.ToEvent(lb))
			}
		}
	} else {
		log.Error("scanning load balancers: %v", err)
	}
	return
}

func (a *AwsApiEventParser) FinishDiscovery() error {
	event := a.NewEvent("discovery")
	event.State = "end"
	return a.SendEvent(event)
}

func (a *AwsApiEventParser) RunForever() {
	connectedEvent := a.NewEvent("bastion")
	connectedEvent.State = "connected"
	connectedEvent.Ttl = 10
	tick := time.Tick(time.Second * 10)
	for {
		a.SendEvent(connectedEvent)
		<-tick
	}
}

func (a *AwsApiEventParser) SendEvent(event *raidman.Event) error {
	log.Debug("%+v", event)
	return a.opseeClient.Send(event)
}

func (a *AwsApiEventParser) NewEvent(service string) *raidman.Event {
	return &raidman.Event{Ttl: defaultEventTtl, Host: a.Hostname, Service: service, Metric: 0, Attributes: make(map[string]string)}
}

func (a *AwsApiEventParser) NewEventWithState(service string, state string) *raidman.Event {
	event := a.NewEvent(service)
	event.State = state
	return event
}

func (e *AwsApiEventParser) ToEvent(obj interface{}) (event *raidman.Event) {
	switch obj.(type) {
	case *ec2.SecurityGroup:
		event = e.ec2SecurityGroupToEvent(obj.(*ec2.SecurityGroup))
	case *elb.LoadBalancerDescription:
		event = e.elbLoadBalancerDescriptionToEvent(obj.(*elb.LoadBalancerDescription))
	case *rds.DBInstance:
		event = e.rdsDBInstanceToEvent(obj.(*rds.DBInstance))
	default:
		event = e.NewEvent("discovery-failure")
		event.State = "failed"
		event.Tags = []string{"failure", "discovery"}
		event.Attributes["api-object-description"] = fmt.Sprint(obj)
		log.Error("unknown API object of type %T:  %+v", obj, obj)

	}
	return
}

func (e *AwsApiEventParser) ec2SecurityGroupToEvent(group *ec2.SecurityGroup) (event *raidman.Event) {
	event = &raidman.Event{Ttl: 120, Host: e.Hostname, Service: "discovery", State: "sg", Metric: 0, Attributes: make(map[string]string)}
	event.State = "sg"
	event.Attributes["group_id"] = *group.GroupID
	if group.GroupName != nil {
		event.Attributes["group_name"] = *group.GroupName
	}
	if len(group.IPPermissions) > 0 {
		perms := group.IPPermissions[0]
		if perms.ToPort != nil {
			event.Attributes["port"] = strconv.Itoa(int(*perms.ToPort))
		}
		if perms.IPProtocol != nil {
			event.Attributes["protocol"] = *perms.IPProtocol
		}
	}
	return
}

func (e *AwsApiEventParser) elbLoadBalancerDescriptionToEvent(lb *elb.LoadBalancerDescription) (event *raidman.Event) {
	event = &raidman.Event{Ttl: 120, Host: e.Hostname, Service: "discovery", State: "rds", Metric: 0, Attributes: make(map[string]string)}
	event.Attributes["group_name"] = *lb.LoadBalancerName
	event.Attributes["group_id"] = *lb.DNSName
	if lb.HealthCheck != nil {
		split := strings.Split(*lb.HealthCheck.Target, ":")
		split2 := strings.Split(split[1], "/")
		event.Attributes["port"] = split2[0]
		event.Attributes["protocol"] = split[0]
		event.Attributes["request"] = strings.Join([]string{"/", split2[1]}, "")
	}
	return
}

func (e *AwsApiEventParser) rdsDBInstanceToEvent(db *rds.DBInstance) (event *raidman.Event) {
	event = &raidman.Event{Ttl: 120, Host: e.Hostname, Service: "discovery", State: "rds", Metric: 0, Attributes: make(map[string]string)}
	if db.DBName != nil {
		event.Attributes["group_name"] = *db.DBName
		if len(db.VPCSecurityGroups) > 0 {
			sgId := *db.VPCSecurityGroups[0].VPCSecurityGroupID
			event.Attributes["group_id"] = sgId
			ec2sg := e.GroupMap[sgId]
			perms := ec2sg.IPPermissions[0]
			event.Attributes["port"] = strconv.Itoa(int(*perms.ToPort))
			event.Attributes["protocol"] = "sql"
			event.Attributes["request"] = "select 1;"
		}
	}
	return
}
