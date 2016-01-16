package checker

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getSlateHost() string {
	return fmt.Sprintf("http://%s/check", os.Getenv("SLATE_HOST"))
}

func testingSlateCheck() *Check {
	return &Check{
		Assertions: []*Assertion{
			&Assertion{
				Key:          "code",
				Relationship: "equal",
				Operand:      "200",
			},
			&Assertion{
				Key:          "body",
				Relationship: "equal",
				Operand:      "A Ok",
			},
		},
	}
}

func testingSlateResponse() *HttpResponse {
	return &HttpResponse{
		Code: int32(200),
		Body: "A Ok",
	}
}

func TestPassingSlateIntegration(t *testing.T) {
	check := testingSlateCheck()
	response := testingSlateResponse()
	client := NewSlateClient(getSlateHost())
	passing, err := client.CheckAssertions(check, response)
	assert.NoError(t, err)
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, passing)
	if !passing {
		t.Fatal("Slate said passing response wasn't passing.")
	}
}

func TestFailingSlateIntegration(t *testing.T) {
	check := testingSlateCheck()
	response := testingSlateResponse()
	response.Code = int32(400)
	client := NewSlateClient(getSlateHost())
	passing, err := client.CheckAssertions(check, response)
	assert.NoError(t, err)
	if err != nil {
		t.Fatal(err)
	}

	assert.False(t, passing)
	if passing {
		t.FailNow()
	}
}
