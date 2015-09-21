package checker

import (
	// "net/http"
	"testing"

	// "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type RunnerTestSuite struct {
	suite.Suite
}

func (s *RunnerTestSuite) RunCheckMultipleTargets() {

}

func TestRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(RunnerTestSuite))
}
