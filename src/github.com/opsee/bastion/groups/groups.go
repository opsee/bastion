package groups

import (
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/opsee/bastion/logging"
	"github.com/streamrail/concurrent-map"
)

var (
	logger = logging.GetLogger("groups")
)

type DynGroup interface {
	GroupId() string
	InstanceEvent(*ec2.Instance)
	GetInstance(id string) *ec2.Instance
}

type expiringInstance struct {
	instance *ec2.Instance
	timer    *time.Timer
}

type remove func()

func newExpiringInstance(ttl time.Duration, instance *ec2.Instance, fn remove) *expiringInstance {
	timer := time.NewTimer(ttl)
	go func() {
		for {
			<-timer.C
			logger.Debug("Expiring from group %+v", instance)
			fn()
		}
	}()
	return &expiringInstance{instance, timer}
}

func (e *expiringInstance) reset(ttl time.Duration) {
	e.timer.Reset(ttl)
}

type sgGroup struct {
	groupId   string
	ttl       time.Duration
	instances cmap.ConcurrentMap
}

func NewSGGroup(groupId string, ttl time.Duration) DynGroup {
	return &sgGroup{groupId, ttl, cmap.New()}
}

func (this *sgGroup) GroupId() string {
	return this.groupId
}

func (this *sgGroup) InstanceEvent(instance *ec2.Instance) {
	id := *instance.InstanceId
	for _, gId := range instance.SecurityGroups {
		if *gId.GroupId == this.groupId {
			exp, ok := this.instances.Get(id)
			if ok {
				expInstance := exp.(*expiringInstance)
				expInstance.reset(this.ttl)
				// we need to do this in case there was a race between the reset and remove
				_, ok2 := this.instances.Get(id)
				if !ok2 {
					this.instances.Set(id, expInstance)
				}
			} else {
				expInstance := newExpiringInstance(this.ttl, instance, func() { this.instances.Remove(id) })
				this.instances.Set(id, expInstance)
			}
			break
		}
	}
}

func (this *sgGroup) GetInstance(id string) *ec2.Instance {
	exp, ok := this.instances.Get(id)
	if ok {
		return exp.(*expiringInstance).instance
	} else {
		return nil
	}
}
