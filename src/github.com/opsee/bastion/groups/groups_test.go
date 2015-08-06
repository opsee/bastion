package groups

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
)

func sendInstance(group DynGroup, instanceId string, groupId string) {
	group.InstanceEvent(&ec2.Instance{InstanceID: aws.String(instanceId),
		SecurityGroups: []*ec2.GroupIdentifier{
			&ec2.GroupIdentifier{GroupID: aws.String(groupId),
				GroupName: aws.String(groupId)}}})
}

func TestSecurityGroupStoresInstances(t *testing.T) {
	group := NewSGGroup("abc", time.Second)
	sendInstance(group, "123", "abc")
	instance := group.GetInstance("123")
	assert.Equal(t, "123", *instance.InstanceID)
}

func TestSecurityGroupDropsExpired(t *testing.T) {
	group := NewSGGroup("abc", time.Second)
	sendInstance(group, "123", "abc")
	time.Sleep(time.Second * 2)
	instance := group.GetInstance("123")
	assert.Nil(t, instance)
}

func TestSecurityGroupFiltersUnOwned(t *testing.T) {
	group := NewSGGroup("abc", time.Second)
	sendInstance(group, "123", "def")
	instance := group.GetInstance("123")
	assert.Nil(t, instance)
}
