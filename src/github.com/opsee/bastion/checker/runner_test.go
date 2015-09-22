package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

type RunnerTestSuite struct {
	suite.Suite
	Common TestCommonStubs
	Runner *Runner
}

func (s *RunnerTestSuite) SetupTest() {
	s.Runner = NewRunner(newTestResolver())
}

func (s *RunnerTestSuite) TestRunCheckMultipleTargets() {
	check := s.Common.PassingCheckMultiTarget()
	responses := s.Runner.RunCheck(context.Background(), check)
	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(CheckResponse), response)
	}
	assert.Equal(s.T(), count, 3)
}

func TestRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(RunnerTestSuite))
}
