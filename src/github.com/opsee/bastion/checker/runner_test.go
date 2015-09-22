package checker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

type RunnerTestSuite struct {
	suite.Suite
	Common   TestCommonStubs
	Runner   *Runner
	Context  context.Context
	Resolver *testResolver
}

func (s *RunnerTestSuite) SetupTest() {
	s.Resolver = newTestResolver()
	s.Runner = NewRunner(s.Resolver)
	s.Common = TestCommonStubs{}
	s.Context = context.Background()
}

func (s *RunnerTestSuite) TestRunCheckHasResponsePerTarget() {
	check := s.Common.PassingCheckMultiTarget()
	responses := s.Runner.RunCheck(s.Context, check)
	targets, err := s.Resolver.Resolve(&Target{
		Id: "sg3",
	})
	assert.NoError(s.T(), err)
	assert.Len(s.T(), targets, 3)

	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(CheckResponse), response)
		assert.NotNil(s.T(), response.Response)
	}
	assert.Equal(s.T(), 3, count)
}

func (s *RunnerTestSuite) TestRunCheckAdheresToMaxHosts() {
	ctx := context.WithValue(s.Context, "MaxHosts", 1)
	check := s.Common.PassingCheckMultiTarget()
	responses := s.Runner.RunCheck(ctx, check)
	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(CheckResponse), response)
		assert.NotNil(s.T(), response.Response)
	}
	assert.Equal(s.T(), 1, count)
}

func (s *RunnerTestSuite) TestRunCheckClosesChannel() {
	check := s.Common.PassingCheckMultiTarget()
	responses := s.Runner.RunCheck(s.Context, check)
	for {
		select {
		case r, ok := <-responses:
			if !ok {
				return
			}
			assert.NotNil(s.T(), r.Response)
		case <-time.After(time.Duration(5) * time.Second):
			assert.Fail(s.T(), "Timed out waiting for response channel to close.")
		}
	}
}

func (s *RunnerTestSuite) TestRunCheckDeadlineExceeded() {
	ctx, _ := context.WithDeadline(s.Context, time.Unix(0, 0))
	check := s.Common.PassingCheckMultiTarget()
	responses := s.Runner.RunCheck(ctx, check)
	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(CheckResponse), response)
		assert.NotNil(s.T(), response.Error)
	}
	assert.Equal(s.T(), 3, count)
}

func (s *RunnerTestSuite) TestRunCheckCancelledContext() {
	ctx, cancel := context.WithCancel(s.Context)
	cancel()
	check := s.Common.PassingCheckMultiTarget()
	responses := s.Runner.RunCheck(ctx, check)
	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(CheckResponse), response)
		assert.NotNil(s.T(), response.Error)
	}
	assert.Equal(s.T(), 3, count)
}

func (s *RunnerTestSuite) TestRunCheckResolveFailureHasOneResponse() {
	check := s.Common.BadCheck()
	responses := s.Runner.RunCheck(s.Context, check)
	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(CheckResponse), response)
		assert.NotNil(s.T(), response.Error)
	}
	assert.Equal(s.T(), 1, count)
}

func (s *RunnerTestSuite) TestRunCheckBadCheckHasOneResponse() {
	check := s.Common.BadCheck()
	responses := s.Runner.RunCheck(s.Context, check)
	count := 0
	for response := range responses {
		count++
		assert.IsType(s.T(), new(CheckResponse), response)
		assert.NotNil(s.T(), response.Error)
	}
	assert.Equal(s.T(), 1, count)
}

func TestRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(RunnerTestSuite))
}
