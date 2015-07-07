package aws

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/opsee/bastion/groups"
	"github.com/opsee/bastion/netutil"
)

const (
	defaultEventTtl = 120
)

type client struct{}

func (c *client) SslOptions() netutil.SslOptions {
	return nil
}

func (c *client) ConnectionMade(baseclient *netutil.BaseClient) bool {
	return true
}

func (c *client) ConnectionLost(bc *netutil.BaseClient, err error) {
}

func (c *client) ReplyReceived(client *netutil.BaseClient, reply *netutil.Message) bool {
	logger.Info("Received message %+v", reply)
	if reply.Command == "healthcheck" {

	}
	return true
}

func MustConnectToOpsee(address string) *netutil.BaseClient {
	connectToOpsee := func() (interface{}, error) { return netutil.ConnectTCP(address, &client{}) }
	connectToOpseeRetrier := netutil.NewBackoffRetrier(connectToOpsee)
	if err := connectToOpseeRetrier.Run(); err != nil {
		logger.Fatalf("connectToOpsee: %v", err)
	}
	return connectToOpseeRetrier.Result().(*netutil.BaseClient)
}

type AwsApiEventParser struct {
	config       *aws.Config
	metadata     *InstanceMeta
	httpClient   *http.Client
	DynGroups    map[string]groups.DynGroup
	GroupMap     map[string]ec2.SecurityGroup
	opseeClient  *netutil.BaseClient
	EC2Client    EC2Scanner
	MessageMaker *netutil.MessageMaker
}

func NewAwsApiEventParser(hostname string, accessKeyId string, secretKey string, region string, customerId string) *AwsApiEventParser {
	httpClient := &http.Client{}
	metap := &MetadataProvider{client: httpClient}
	hostname = netutil.GetHostnameDefault(hostname)
	var metadata *InstanceMeta = nil
	if region == "" {
		metadata = metap.Get()
		region = metadata.Region
	} else {
		//if we're passing region in on the cmd line it means we're running outside of aws
		metadata = &InstanceMeta{InstanceId: hostname}
	}
	var creds = credentials.NewChainCredentials(
		[]credentials.Provider{
			&credentials.StaticProvider{Value: credentials.Value{
				AccessKeyID:     accessKeyId,
				SecretAccessKey: secretKey,
				SessionToken:    "",
			}},
			&credentials.EnvProvider{},
			&credentials.EC2RoleProvider{ExpiryWindow: 5 * time.Minute},
		})
	config := &aws.Config{Credentials: creds, Region: region}
	scanner := &AwsApiEventParser{
		config:       config,
		metadata:     metadata,
		httpClient:   httpClient,
		DynGroups:    make(map[string]groups.DynGroup),
		GroupMap:     make(map[string]ec2.SecurityGroup),
		EC2Client:    NewScanner(config),
		MessageMaker: netutil.NewMessageMaker(defaultEventTtl, metadata.InstanceId, hostname, customerId),
	}
	return scanner
}

func (a *AwsApiEventParser) ConnectToOpsee(address string) {
	a.opseeClient = MustConnectToOpsee(address)
	go a.opseeClient.Loop()
	a.sendConnectedEvent()
}

func (a *AwsApiEventParser) Scan() (err error) {
	defer func() { a.sendDiscoveryStateEvent("end") }()
	a.sendDiscoveryStateEvent("start")
	if err = a.ScanSecurityGroups(); err == nil {
		if err = a.ScanLoadBalancers(); err == nil {
			if err = a.ScanRDS(); err != nil {
				logger.Error("AwsApiEventParser.ScanRDS: %v", err)
			}
		} else {
			logger.Error("AwsApiEventParse.ScanLoadBalancers: %v", err)
		}
	} else {
		logger.Error("AwsApiEventParser.ScanSecurityGroup: %v", err)
	}
	return
}

func (a *AwsApiEventParser) ScanSecurityGroups() (err error) {
	if groups, err := a.EC2Client.ScanSecurityGroups(); err != nil {
		logger.Error("scanning security groups: %s", err.Error())
	} else {
		for _, group := range groups {
			if group.GroupID != nil {
				a.GroupMap[*group.GroupID] = *group
				reservations, _ := a.EC2Client.ScanSecurityGroupInstances(*group.GroupID)
				for _, reservation := range reservations {
					for _, instance := range reservation.Instances {
						a.SendEvent(a.ToEvent(instance))
					}
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
		logger.Error("ScanRDS: %v", err)
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
				err = a.SendEvent(a.ToEvent(lb))
			}
		}
	} else {
		logger.Error("scanning load balancers: %v", err)
	}
	return
}

func (a *AwsApiEventParser) sendDiscoveryStateEvent(state string) error {
	if state != "start" || state != "end" {
		return errors.New("invalid discovery event %s (expected 'start' or 'end')")
	}
	event := a.MessageMaker.NewMessage()
	event.Command = "discovery"
	event.Attributes["state"] = state
	return a.SendEvent(event)
}

func (a *AwsApiEventParser) sendConnectedEvent() error {
	connectedEvent := a.MessageMaker.NewMessage()
	connectedEvent.Command = "connected"
	connectedEvent.Attributes["state"] = "connected"
	connectedEvent.Ttl = 10
	return a.SendEvent(connectedEvent)
}

func (a *AwsApiEventParser) RunForever() {
	tick := time.Tick(time.Second * 10)
	for {
		a.sendConnectedEvent()
		<-tick
	}
}

func (a *AwsApiEventParser) SendEvent(event *netutil.Message) error {
	return a.opseeClient.SendEvent(event)
}

func (a *AwsApiEventParser) ToEvent(obj interface{}) (event *netutil.Message) {
	switch obj.(type) {
	case *ec2.SecurityGroup:
		event = a.ec2SecurityGroupToEvent(obj.(*ec2.SecurityGroup))
	case *elb.LoadBalancerDescription:
		event = a.elbLoadBalancerDescriptionToEvent(obj.(*elb.LoadBalancerDescription))
	case *rds.DBInstance:
		event = a.rdsDBInstanceToEvent(obj.(*rds.DBInstance))
	case *ec2.Instance:
		event = a.ec2InstanceToEvent(obj.(*ec2.Instance))
	default:
		event = a.MessageMaker.NewMessage()
		event.Command = "discovery-failure"
		event.Attributes["api-object-description"] = fmt.Sprint(obj)
		event.Attributes["tags"] = []string{"failure", "discovery"}
		logger.Error("unknown API object of type %T:  %+v", obj, obj)
	}
	return
}

func (e *AwsApiEventParser) ec2SecurityGroupToEvent(group *ec2.SecurityGroup) (event *netutil.Message) {
	event = e.MessageMaker.NewMessage()
	event.Command = "discovery"
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

func (e *AwsApiEventParser) elbLoadBalancerDescriptionToEvent(lb *elb.LoadBalancerDescription) (event *netutil.Message) {
	event = e.MessageMaker.NewMessage()
	event.Command = "discovery"
	event.Attributes["group_name"] = *lb.LoadBalancerName
	event.Attributes["group_id"] = *lb.DNSName
	if lb.HealthCheck != nil {
		// TCP:port
		// SSL:port
		// HTTP:port;/;PathToPing; grouping, for example "HTTP:80/weather/us/wa/seattle"
		split := strings.Split(*lb.HealthCheck.Target, ":")
		split2 := strings.Split(split[1], "/")
		logger.Info("split %+v split2 %+v", split, split2)
		event.Attributes["port"] = split2[0]
		event.Attributes["interval"] = string(*lb.HealthCheck.Interval)
		event.Attributes["protocol"] = split[0]
		if len(split2) > 1 {
			event.Attributes["request"] = strings.Join([]string{"/", split2[1]}, "")
		}
	}
	return
}

func (a *AwsApiEventParser) ec2InstanceToEvent(instance *ec2.Instance) (event *netutil.Message) {
	event = a.MessageMaker.NewMessage()
	event.Command = "discovery"
	event.Attributes["instance"] = instance
	logger.Debug("security groups: %v", instance.SecurityGroups)
	return
}

func (e *AwsApiEventParser) rdsDBInstanceToEvent(db *rds.DBInstance) (event *netutil.Message) {
	event = e.MessageMaker.NewMessage()
	event.Command = "discovery"
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
