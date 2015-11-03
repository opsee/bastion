package checker

import (
	"github.com/Sirupsen/logrus"
	"os"
	"testing"
)

func TestGetChecks(t *testing.T) {
	ba := &BastionAuthCache{tokenMap: make(map[BastionAuthTokenType]*BastionAuthToken)}

	authType := ba.resolveAuthType(os.Getenv("BASTION_AUTH_TYPE"))
	endpoint := os.Getenv("BARTNET_ENDPOINT") + "/checks"

	if token, err := ba.getToken(authType, endpoint); err != nil || token == nil {
		logrus.WithFields(logrus.Fields{"service": "checker", "error": err.Error()}).Error()
	}
}
