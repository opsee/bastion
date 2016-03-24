package auth

import (
	"fmt"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/opsee/bastion/config"
)

func TestGetBasicToken(t *testing.T) {
	cache := &BastionAuthCache{Tokens: make(map[string]*BastionAuthToken)}

	tokenType, err := GetTokenTypeByString("BASIC_TOKEN")
	if err != nil {
		logrus.WithFields(logrus.Fields{"service": "auth", "error": err.Error()}).Error("Error getting auth token")
		t.FailNow()
	}

	request := &BastionAuthTokenRequest{
		TokenType:      tokenType,
		CustomerEmail:  config.GetConfig().CustomerEmail,
		CustomerID:     config.GetConfig().CustomerId,
		TargetEndpoint: fmt.Sprintf("%s/checks", config.GetConfig().BartnetHost),
		AuthEndpoint:   config.GetConfig().BastionAuthEndpoint,
	}

	if token, err := cache.GetToken(request); err != nil || token == nil {
		logrus.WithFields(logrus.Fields{"service": "auth", "error": err.Error()}).Error("Error getting auth token")
		t.FailNow()
	} else {
		logrus.WithFields(logrus.Fields{"service": "auth", "token": token}).Info("got BASIC_TOKEN")
	}
}
