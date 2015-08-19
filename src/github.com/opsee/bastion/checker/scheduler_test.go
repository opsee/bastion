package checker

import (
	"testing"
)

// Test the Scheduler

func testCheckStub() *Check {
	return &Check{
		Id:        "string",
		Interval:  60,
		Target:    &Target{},
		CheckSpec: &Any{},
	}
}

func testScheduler() *Scheduler {
	return NewScheduler()
}

// I am lazy, so I am only testing validateCheck once.

/*******************************************************************************
 * validateCheck()
 ******************************************************************************/

func TestSchedulerValidateGoodCheck(t *testing.T) {
	check := testCheckStub()

	if err := validateCheck(check); err != nil {
		t.Fail()
	}
}

func TestSchedulerValidateWithoutId(t *testing.T) {
	check := testCheckStub()
	check.Id = ""

	if err := validateCheck(check); err == nil {
		t.Fail()
	}
}

func TestSchedulerValidateInvalidInterval(t *testing.T) {
	check := testCheckStub()
	check.Interval = 0

	if err := validateCheck(check); err == nil {
		t.Fail()
	}
}

func TestSchedulerValidateWithoutTarget(t *testing.T) {
	check := testCheckStub()
	check.Target = nil

	if err := validateCheck(check); err == nil {
		t.Fail()
	}
}

func TestSchedulerValidateWithoutCheckSpec(t *testing.T) {
	check := testCheckStub()
	check.CheckSpec = nil

	if err := validateCheck(check); err == nil {
		t.Fail()
	}
}

/*******************************************************************************
 * CreateCheck()
 ******************************************************************************/

// I'm okay with this testing both successful creates and retrieves.
func TestSchedulerCreateCheckStoresCheck(t *testing.T) {
	scheduler := testScheduler()
	check := testCheckStub()
	id := check.Id

	scheduler.CreateCheck(testCheckStub())

	c, err := scheduler.RetrieveCheck(id)

	if err != nil {
		t.Fail()
	}

	if c == nil {
		t.FailNow()
	}

	if c.Id != id {
		t.Fail()
	}
}

/*******************************************************************************
 * RetrieveCheck()
 ******************************************************************************/

func TestSchedulerRetrieveNonexistentCheck(t *testing.T) {
	scheduler := testScheduler()

	c, err := scheduler.RetrieveCheck("id string")
	if c != nil {
		t.Fail()
	}

	if err == nil {
		t.Fail()
	}
}

/*******************************************************************************
 * DeleteCheck()
 ******************************************************************************/

func TestSchedulerDeleteNonexistentCheck(t *testing.T) {
	scheduler := testScheduler()

	c, err := scheduler.DeleteCheck("id string")
	if c != nil {
		t.Fail()
	}

	if err == nil {
		t.Fail()
	}
}

func TestSchedulerDeleteReturnsOriginalCheck(t *testing.T) {
	scheduler := testScheduler()
	check := testCheckStub()
	scheduler.CreateCheck(check)

	scheduler.CreateCheck(check)
	c, err := scheduler.DeleteCheck(check.Id)

	if c == nil {
		t.FailNow()
	}

	if err != nil {
		t.Fail()
	}

	if c != check {
		t.Fail()
	}
}

// Test the concurrent map
