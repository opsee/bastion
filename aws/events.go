package aws

import (
    "errors"
    "fmt"
    "github.com/awslabs/aws-sdk-go/service/ec2"
    "github.com/awslabs/aws-sdk-go/service/elb"
    "github.com/awslabs/aws-sdk-go/service/rds"
    "github.com/opsee/bastion/netutil"
    "net/http"
    "strconv"
    "strings"
    "time"
)

const (
    defaultEventTtl = 120
)

type client struct {}

func (c *client) SslOptions() netutil.SslOptions {
    return nil
}

func (c *client) ConnectionMade(baseclient *netutil.BaseClient) bool {
    return true
}

func (c *client) ConnectionLost(bc *netutil.BaseClient, err error) {
}

func (c *client) ReplyReceived(client *netutil.BaseClient, reply *netutil.EventMessage) bool {
    return true
}

func MustConnectToOpsee(address string) *netutil.BaseClient {
    connectToOpsee := func() (interface{}, error) { return netutil.ConnectTCP(address, &client{}) }
    connectToOpseeRetrier := netutil.NewBackoffRetrier(connectToOpsee)
    if err := connectToOpseeRetrier.Run(); err != nil {
        log.Fatalf("connectToOpsee: %v", err)
    }
    return connectToOpseeRetrier.Result().(*netutil.BaseClient)
}

type AwsApiEventParser struct {
    *CredentialsProvider
    hostname     string
    instanceId   string
    accessKeyId  string
    secretKey    string
    region       string
    httpClient   *http.Client
    GroupMap     map[string]ec2.SecurityGroup
    opseeClient  *netutil.BaseClient
    EC2Client    EC2Scanner
    MessageMaker *netutil.EventMessageMaker
}

func NewAwsApiEventParser(hostname string, accessKeyId string, secretKey string, region string) *AwsApiEventParser {
    httpClient := &http.Client{}
    credProvider := NewProvider(httpClient, accessKeyId, secretKey, region)
    instanceId := ""
    if credProvider.GetInstanceId() != nil {
        instanceId = credProvider.GetInstanceId().InstanceId
    }
    scanner := &AwsApiEventParser{
        CredentialsProvider: credProvider,
        hostname:            netutil.GetHostnameDefault(instanceId),
        instanceId:          instanceId,
        accessKeyId:         accessKeyId,
        secretKey:           secretKey,
        region:              region,
        httpClient:          httpClient,
        GroupMap:            make(map[string]ec2.SecurityGroup),
        EC2Client:           NewScanner(credProvider),
        MessageMaker:        netutil.NewEventMessageMaker(defaultEventTtl, instanceId, hostname),
    }
    return scanner
}

func (a *AwsApiEventParser) ConnectToOpsee(address string) {
    a.opseeClient = MustConnectToOpsee(address)
    a.sendConnectedEvent()
}

func (a *AwsApiEventParser) Scan() (err error) {
    defer func() { a.sendDiscoveryStateEvent("end") }()
    a.sendDiscoveryStateEvent("start")
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
                err = a.SendEvent(a.ToEvent(lb))
            }
        }
    } else {
        log.Error("scanning load balancers: %v", err)
    }
    return
}

func (a *AwsApiEventParser) sendDiscoveryStateEvent(state string) error {
    if state != "start" || state != "end" {
        return errors.New("invalid discovery event %s (expected 'start' or 'end')")
    }
    event := a.MessageMaker.NewEventMessage()
    event.Command = "discovery"
    event.State = state
    return a.SendEvent(event)
}

func (a *AwsApiEventParser) sendConnectedEvent() error {
    connectedEvent := a.MessageMaker.NewEventMessage()
    connectedEvent.Command = "connected"
    connectedEvent.State = "connected"
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

func (a *AwsApiEventParser) SendEvent(event *netutil.EventMessage) error {
    return a.opseeClient.SendEvent(event)
}

func (a *AwsApiEventParser) ToEvent(obj interface{}) (event *netutil.EventMessage) {
    switch obj.(type) {
        case *ec2.SecurityGroup:
        event = a.ec2SecurityGroupToEvent(obj.(*ec2.SecurityGroup))
        case *elb.LoadBalancerDescription:
        event = a.elbLoadBalancerDescriptionToEvent(obj.(*elb.LoadBalancerDescription))
        case *rds.DBInstance:
        event = a.rdsDBInstanceToEvent(obj.(*rds.DBInstance))
        default:
        event = a.MessageMaker.NewEventMessage()
        event.Command = "discovery-failure"
        event.State = "failed"
        event.Tags = []string{"failure", "discovery"}
        event.Attributes["api-object-description"] = fmt.Sprint(obj)
        log.Error("unknown API object of type %T:  %+v", obj, obj)
    }
    return
}

func (e *AwsApiEventParser) ec2SecurityGroupToEvent(group *ec2.SecurityGroup) (event *netutil.EventMessage) {
    event = e.MessageMaker.NewEventMessage()
    event.Service = "discovery"
    event.Command = "discovery"
    event.State = "sg"
    event.Metric = 0
    event.Attributes["group_id"] = *group.GroupID
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

func (e *AwsApiEventParser) elbLoadBalancerDescriptionToEvent(lb *elb.LoadBalancerDescription) (event *netutil.EventMessage) {
    event = e.MessageMaker.NewEventMessage()
    event.Command = "discovery"
    event.Service = "discovery"
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

func (e *AwsApiEventParser) rdsDBInstanceToEvent(db *rds.DBInstance) (event *netutil.EventMessage) {
    event = e.MessageMaker.NewEventMessage()
    event.Command = "discovery"
    event.Service = "discovery"
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
