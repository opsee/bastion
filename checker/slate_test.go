package checker

import (
	"encoding/json"
	"fmt"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/basic/schema"
	"github.com/opsee/bastion/config"
	opsee_types "github.com/opsee/protobuf/opseeproto/types"
	"golang.org/x/net/context"
)

func getSlateHost() string {
	return fmt.Sprintf("http://%s/check", config.GetConfig().SlateHost)
}

// this should cover true and false cases for each check type
var SlateTests = []struct {
	check    *schema.Check
	response interface{}
	expected bool
}{
	{
		&schema.Check{
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
		},
		&schema.HttpResponse{
			Code: int32(200),
			Body: "A Ok",
		},
		true,
	},
	{
		&schema.Check{
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
		},
		&schema.HttpResponse{
			Code: int32(500),
			Body: "Internal Server Error",
		},
		false,
	},
	{
		&schema.Check{
			Assertions: []*schema.Assertion{
				&schema.Assertion{
					Key:          "cloudwatch",
					Value:        "CPUUtilization",
					Relationship: "lessThan",
					Operand:      "95",
				},
				&schema.Assertion{
					Key:          "cloudwatch",
					Value:        "CPUUtilization",
					Relationship: "greaterThan",
					Operand:      "60",
				},
			},
		},
		&schema.CloudWatchResponse{
			Namespace: "AWS/RDS",
			Metrics: []*schema.Metric{
				&schema.Metric{
					Name:      "CPUUtilization",
					Value:     79,
					Timestamp: &opsee_types.Timestamp{},
					Tags:      []*schema.Tag{},
				},
				&schema.Metric{
					Name:      "CPUUtilization",
					Value:     89,
					Timestamp: &opsee_types.Timestamp{},
					Tags:      []*schema.Tag{},
				},
			},
		},
		true,
	},
	{
		&schema.Check{
			Assertions: []*schema.Assertion{
				&schema.Assertion{
					Key:          "cloudwatch",
					Value:        "CPUUtilization",
					Relationship: "greaterThan",
					Operand:      "10",
				},
				&schema.Assertion{
					Key:          "cloudwatch",
					Value:        "CPUUtilization",
					Relationship: "lessThan",
					Operand:      "95",
				},
				&schema.Assertion{
					Key:          "cloudwatch",
					Value:        "ReadIOPS",
					Relationship: "greaterThan",
					Operand:      "10",
				},
				&schema.Assertion{
					Key:          "cloudwatch",
					Value:        "ReadIOPS",
					Relationship: "lessThan",
					Operand:      "200",
				},
			},
		},
		&schema.CloudWatchResponse{
			Metrics: []*schema.Metric{
				&schema.Metric{
					Name:      "CPUUtilization",
					Value:     100,
					Timestamp: &opsee_types.Timestamp{},
					Tags:      []*schema.Tag{},
					Unit:      "Percent",
					Statistic: "Average",
				},
				&schema.Metric{
					Name:      "ReadIOPS",
					Value:     100,
					Timestamp: &opsee_types.Timestamp{},
					Tags:      []*schema.Tag{},
					Unit:      "Count/Second",
					Statistic: "Average",
				},
			},
		},
		false,
	},
	{
		&schema.Check{
			Assertions: []*schema.Assertion{
				&schema.Assertion{
					Key:          "cloudwatch",
					Value:        "CPUUtilization",
					Relationship: "lessThan",
					Operand:      "100",
				},
			},
		},
		// Test to see if zero value is preserved in Metric JSON.
		&schema.CloudWatchResponse{
			Metrics: []*schema.Metric{
				&schema.Metric{
					Name:      "CPUUtilization",
					Value:     0,
					Timestamp: &opsee_types.Timestamp{},
					Tags:      []*schema.Tag{},
					Unit:      "Percent",
					Statistic: "Average",
				},
			},
		},
		true,
	},
}

func TestSlateTests(t *testing.T) {
	client := NewSlateClient(getSlateHost())
	for i, test := range SlateTests {
		response, err := json.Marshal(test.response)
		if err != nil {
			log.WithError(err).Fatal("Failed to marshal json response")
		}
		log.WithFields(log.Fields{"response": string(response)}).Debug("Testing response")
		actual, err := client.CheckAssertions(context.Background(), test.check, response)
		if err != nil {
			t.Fatal(err)
		}
		if actual != test.expected {
			log.Fatalf("Test[%d]: expected %t, actual %t", i, test.expected, actual)
		}
	}
}
