package main

import "testing"

func TestHackerValidateFail(t *testing.T) {
	hacker := &Hacker{}
	err := hacker.Validate()
	if err == nil {
		t.FailNow()
	}
	t.Log(err)
}

/*
func TestHackerCanGetSecurityGroups(t *testing.T) {
	hacker, err := NewHacker()
	if err != nil {
		t.FailNow()
	}
	groups, err := hacker.GetSecurityGroups()
	if err != nil {
		t.FailNow()
	}
	for _, group := range groups {
		logrus.Info(*group.GroupName)
	}
}

func TestHackerCanGetStackTemplateBody(t *testing.T) {
	hacker, err := NewHacker()
	if err != nil {
		t.FailNow()
	}
	body, err := hacker.GetStackTemplateBody("opsee-stack-5963d7bc-6ba2-11e5-8603-6ba085b2f5b5")
	if err != nil {
		t.FailNow()
	}
	logrus.Info(*body)
}
func TestHackerCanCreateStack(t *testing.T) {
	hacker, err := NewHacker()
	if err != nil {
		t.FailNow()
	}

	template := cf.NewTemplate()
	template.Description = "Listing of bastion security-group ingress rules."
	template.Parameters["BastionId"] = &cf.Parameter{
		Description: "The top level DNS name for the infrastructure",
		Type:        "String",
	}

	template.AddResource("TestSGIngressRule", cf.EC2SecurityGroupIngress{
		IpProtocol:            cf.String("tcp"),
		FromPort:              cf.Integer(0),
		ToPort:                cf.Integer(65535),
		SourceSecurityGroupId: cf.String("sg-e0a5a385"),
		GroupId:               cf.String("sg-f2c2ef97"),
	})

	templateBody, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		t.FailNow()
	}

	parameters := []*cloudformation.Parameter{
		&cloudformation.Parameter{
			ParameterKey:   aws.String("BastionId"),
			ParameterValue: aws.String("opsee-test-cfn-bastion-id"),
		},
	}

	stackId, err := hacker.CreateStack("hacker-test-bastion", parameters, string(templateBody), int64(5))
	if err != nil {
		logrus.WithError(err).Error("Failed to create stack")
		t.FailNow()
	}
	logrus.Info(*stackId)
}

func TestHackerCanHack(t *testing.T) {
	hacker, err := NewHacker()
	if err != nil {
		t.FailNow()
	}
	_, err = hacker.Hack()
	if err != nil {
		t.Log("failed to create stack")
		t.FailNow()
	}
}
*/
