package checker

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/opsee/basic/schema"
)

func NewCheckTargets(resolver Resolver, check *schema.Check) (*schema.CheckTargets, error) {
	if check.Target == nil {
		return nil, fmt.Errorf("resolveRequestTargets: Check requires target. CHECK=%#v", check)
	}

	targets, err := resolver.Resolve(context.TODO(), check.Target)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("No valid targets resolved from %s", check.Target)
	}

	return &schema.CheckTargets{
		Check:   check,
		Targets: targets,
	}, nil
}
