package checker

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/opsee/basic/schema"
)

type CheckWithTargets struct {
	Check   *schema.Check
	Targets []*schema.Target
}

func NewCheckWithTargets(resolver Resolver, check *schema.Check) (*CheckWithTargets, error) {
	//TODO(dan) context stuff
	ctx := context.Background()
	if check.Target == nil {
		return nil, fmt.Errorf("resolveRequestTargets: Check requires target. CHECK=%s", check)
	}

	targets, err := resolver.Resolve(ctx, check.Target)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("No valid targets resolved from %s", check.Target)
	}

	return &CheckWithTargets{
		Check:   check,
		Targets: targets,
	}, nil
}
