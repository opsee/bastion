package checker

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/opsee/basic/schema"
)

type CheckWithTargets struct {
	CheckProto *proto.Message
	Targets    []*schema.Target
}

func NewCheckWithTargets(awsResolver *AWSResolver, check *schema.Check) (*CheckWithTargets, error) {
	if check.Target == nil {
		return nil, fmt.Errorf("resolveRequestTargets: Check requires target. CHECK=%s", check)
	}

	targets, err = awsResolver.Resolve(ctx, check.Target)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("No valid targets resolved from %s", check.Target)
	}
	msg, err := proto.Marshal(check)
	if err != nil {
		return nil, err
	}

	return &CheckWithTargets{
		CheckProto: msg,
		Targets:    targets,
	}, nil
}
