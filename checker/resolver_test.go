package checker

import (
	"testing"

	"github.com/opsee/basic/schema"
	"github.com/stretchr/testify/assert"
)

func TestResolveHost(t *testing.T) {
	var (
		assert   = assert.New(t)
		resolver = &AWSResolver{}
		targets  []*schema.Target
		err      error
	)

	targets, err = resolver.resolveHost("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(1, len(targets), "resolver.resolveHost will work with an ip address given as a parameter")
	assert.EqualValues(
		&schema.Target{Type: "host", Id: "127.0.0.1", Address: "127.0.0.1"},
		targets[0],
		"resolver.resolveHost will work with an ip address given as a parameter",
	)

	targets, err = resolver.resolveHost("reddit.com")
	if err != nil {
		t.Fatal(err)
	}

	assert.True(len(targets) > 1, "resolver.resolveHost will resolve multiple ip targets")
	for _, t := range targets {
		assert.NotEmpty(t.Address, "resolver.resolveHost will resolve multiple ip targets")
	}
}
