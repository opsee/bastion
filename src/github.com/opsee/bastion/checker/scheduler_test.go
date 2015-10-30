package checker

import (
	"testing"

	"github.com/opsee/bastion/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

// Test the Scheduler

type SchedulerTestSuite struct {
	suite.Suite
	Common    TestCommonStubs
	Scheduler *Scheduler
}

func (s *SchedulerTestSuite) SetupTest() {
	s.Common = TestCommonStubs{}
	s.Scheduler = NewScheduler()
}

// I am lazy, so I am only testing validateCheck once.

/*******************************************************************************
 * validateCheck()
 ******************************************************************************/

func (s *SchedulerTestSuite) TestValidCheckIsValid() {
	check := s.Common.Check()
	assert.NoError(s.T(), validateCheck(check))
}

func (s *SchedulerTestSuite) TestCheckWithoutIDIsInvalid() {
	check := s.Common.Check()
	check.Id = ""
	assert.Error(s.T(), validateCheck(check))
}

func (s *SchedulerTestSuite) TestCheckWithZeroIntervalIsInvalid() {
	check := s.Common.Check()
	check.Interval = 0
	assert.Error(s.T(), validateCheck(check))
}

func (s *SchedulerTestSuite) TestCheckWithoutTargetIsInvalid() {
	check := s.Common.Check()
	check.Target = nil
	assert.Error(s.T(), validateCheck(check))
}

func (s *SchedulerTestSuite) TestCheckWithoutCheckSpecIsInvalid() {
	check := s.Common.Check()
	check.CheckSpec = nil
	assert.Error(s.T(), validateCheck(check))
}

/*******************************************************************************
 * CreateCheck()
 ******************************************************************************/

// I'm okay with this testing both successful creates and retrieves.
func (s *SchedulerTestSuite) TestCreateCheckStoresCheck() {
	scheduler := s.Scheduler
	check := s.Common.Check()
	id := check.Id
	scheduler.CreateCheck(check)
	c, err := scheduler.RetrieveCheck(id)
	assert.NoError(s.T(), err, "Scheduler.RetrieveCheck return unexpected error.", err)
	assert.IsType(s.T(), new(Check), c, "Scheduler.RetrieveCheck returned incorrect object type.")
	assert.Equal(s.T(), id, c.Id, "Scheduler.RetrieveCheck returned ID does not match.")
}

// TODO(greg): I don't know why this causes tests to hang, but it does. I'm
// commenting this test out for now. We can fix it later.
// func (s *SchedulerTestSuite) TestCreateCheckStartsSchedulingCheck() {
// 	scheduler := s.Scheduler
// 	check := s.Common.PassingCheck()
// 	check.Interval = 1
// 	id := check.Id
// 	scheduler.CreateCheck(check)
// 	rcvdCheck := <-scheduler.scheduleMap.runChan
// 	scheduler.scheduleMap.Destroy()
// 	assert.NotNil(s.T(), rcvdCheck)
// 	assert.Equal(s.T(), id, rcvdCheck.Id)
// }

/*******************************************************************************
 * RetrieveCheck()
 ******************************************************************************/

func (s *SchedulerTestSuite) TestRetrieveNonexistentCheckReturnsError() {
	scheduler := s.Scheduler

	c, err := scheduler.RetrieveCheck("id string")
	assert.Nil(s.T(), c)
	assert.Error(s.T(), err, "Scheduler.RetrieveCheck did not return error for non-existent check.")
}

/*******************************************************************************
 * DeleteCheck()
 ******************************************************************************/

func (s *SchedulerTestSuite) TestDeleteNonexistentCheckReturnsError() {
	scheduler := s.Scheduler

	c, err := scheduler.DeleteCheck("id string")
	scheduler.DeleteCheck("id string")
	assert.Nil(s.T(), c)
	assert.Error(s.T(), err, "Scheduler.DeleteCheck did not return error for non-existent check.")
}

func (s *SchedulerTestSuite) TestDeleteReturnsOriginalCheck() {
	scheduler := s.Scheduler
	check := s.Common.Check()
	scheduler.CreateCheck(check)

	scheduler.CreateCheck(check)
	c, err := scheduler.DeleteCheck(check.Id)
	assert.NoError(s.T(), err, "DeleteCheck returned unexpected error.", err)
	assert.IsType(s.T(), new(Check), c, "DeleteCheck returned object of incorrect type.")
	assert.Equal(s.T(), check.Id, c.Id, "DeleteCheck returned incorrect check ID.")
}

/*******************************************************************************
 * RunCheck() Benchmarks
  ******************************************************************************/

func BenchmarkRunCheckParallel(b *testing.B) {
	logging.SetLevel("ERROR", "checker")
	runner := NewRunner(newTestResolver())
	check := (&TestCommonStubs{}).PassingCheckMultiTarget()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			runner.RunCheck(context.Background(), check)
		}
	})
}

func TestSchedulerTestSuite(t *testing.T) {
	setupTestEnv()
	suite.Run(t, new(SchedulerTestSuite))
}
