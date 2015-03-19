package scanner
import (
    "github.com/amir/raidman"
    "github.com/awslabs/aws-sdk-go/gen/ec2"
    "github.com/awslabs/aws-sdk-go/gen/elb"
    "github.com/awslabs/aws-sdk-go/gen/rds"
    "strconv"
    "strings"
)

type AwsApiEventParser struct {
    Hostname string
}

func NewAwsApiEventParser(hostname string) (parser *AwsApiEventParser) {
    return &AwsApiEventParser{hostname}
}

func (e *AwsApiEventParser) NewEvent(service string) (*raidman.Event) {
    return &raidman.Event{Ttl: 120, Host: e.Hostname, Service: service, Metric: 0, Attributes: make(map[string]string)}
}

func (e *AwsApiEventParser) ToEvent(obj interface{}) (event *raidman.Event) {
    switch obj.(type) {
        case ec2.SecurityGroup:
            event = e.ec2SecurityGroupToEvent(obj.(ec2.SecurityGroup))
        case elb.LoadBalancerDescription:
            event = e.elbLoadBalancerDescriptionToEvent(obj.(elb.LoadBalancerDescription))
        case rds.DBInstance:
            event = e.rdsDBInstanceToEvent(obj.(rds.DBInstance))
        default:
            log.Error("unknown API object: ", obj)
    }
    return
}

func (e *AwsApiEventParser) ec2SecurityGroupToEvent(group ec2.SecurityGroup) (event *raidman.Event) {
    event = &raidman.Event{Ttl: 120, Host: e.Hostname, Service: "discovery", State: "sg", Metric: 0, Attributes: make(map[string]string)}
    event.State = "sg"
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
    return
}

func (e *AwsApiEventParser) elbLoadBalancerDescriptionToEvent(lb elb.LoadBalancerDescription) (event *raidman.Event) {
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

func (e *AwsApiEventParser) rdsDBInstanceToEvent(db rds.DBInstance) (event *raidman.Event) {
    event = &raidman.Event{Ttl: 120, Host: e.Hostname, Service: "discovery", State: "rds", Metric: 0, Attributes: make(map[string]string)}
    if db.DBName != nil {
        event.Attributes["group_name"] = *db.DBName
        if len(db.VPCSecurityGroups) > 0 {
            sgId := *db.VPCSecurityGroups[0].VPCSecurityGroupID
            event.Attributes["group_id"] = sgId
            // XXX TODO ec2sg := groupMap[sgId]
            // XXX TODO perms := ec2sg.IPPermissions[0]
            // XXX TODO event.Attributes["port"] = strconv.Itoa(*perms.ToPort)
            event.Attributes["protocol"] = "sql"
            event.Attributes["request"] = "select 1;"
        }
    }
    return event
}
