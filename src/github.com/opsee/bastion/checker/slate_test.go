package checker

import (
	"fmt"
	"os"
	"testing"

	"github.com/opsee/basic/schema"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func getSlateHost() string {
	return fmt.Sprintf("http://%s/check", os.Getenv("SLATE_HOST"))
}

func testingSlateCheck() *schema.Check {
	return &schema.Check{
		Assertions: []*schema.Assertion{
			&schema.Assertion{
				Key:          "code",
				Relationship: "equal",
				Operand:      "200",
			},
			&schema.Assertion{
				Key:          "body",
				Relationship: "equal",
				Operand:      "A Ok",
			},
		},
	}
}

func testingSlateResponse() *schema.HttpResponse {
	return &schema.HttpResponse{
		Code: int32(200),
		Body: "A Ok",
	}
}

func TestPassingSlateIntegration(t *testing.T) {
	check := testingSlateCheck()
	response := testingSlateResponse()
	client := NewSlateClient(getSlateHost())
	passing, err := client.CheckAssertions(context.Background(), check, response)
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
	passing, err := client.CheckAssertions(context.Background(), check, response)
	assert.NoError(t, err)
	if err != nil {
		t.Fatal(err)
	}

	assert.False(t, passing)
	if passing {
		t.FailNow()
	}
}
