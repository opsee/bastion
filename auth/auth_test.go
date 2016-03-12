package auth

import (
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
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
		CustomerEmail:  os.Getenv("CUSTOMER_EMAIL"),
		CustomerID:     os.Getenv("CUSTOMER_ID"),
		TargetEndpoint: "https://bartnet.in.opsee.com/checks",
		AuthEndpoint:   os.Getenv("BASTION_AUTH_ENDPOINT"),
	}

	if token, err := cache.GetToken(request); err != nil || token == nil {
		logrus.WithFields(logrus.Fields{"service": "auth", "error": err.Error()}).Error("Error getting auth token")
		t.FailNow()
	} else {
		logrus.WithFields(logrus.Fields{"service": "auth", "token": token}).Info("got BASIC_TOKEN")
	}
}
