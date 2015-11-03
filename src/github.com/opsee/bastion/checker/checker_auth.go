package checker

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/Sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type BastionAuthTokenType string

const (
	bearerToken = "Bearer"
	basicToken  = "Basic"
)

type BastionAuthToken struct {
	Type     BastionAuthTokenType
	Token    string
	Endpoint string
}

type BastionAuthCache struct {
	email             string
	password          string
	customerId        string
	customerIdEncoded string

	tokenMap map[BastionAuthTokenType]*BastionAuthToken
}

func (ba *BastionAuthCache) resolveAuthType(t string) BastionAuthTokenType {
	t = strings.ToLower(t)
	switch t {
	case "bearer":
		return bearerToken
	case "basic":
		return basicToken
	default:
		return basicToken
	}
}

func (ba *BastionAuthCache) getToken(tokenType BastionAuthTokenType, endpoint string) (*BastionAuthToken, error) {
	newToken := &BastionAuthToken{}

	switch tokenType {
	case bearerToken:
		ba.email = os.Getenv("CUSTOMER_EMAIL")
		ba.password = os.Getenv("CUSTOMER_PASSWORD")
		if len(ba.email) > 0 && len(ba.password) > 0 {
			url := os.Getenv("BASTION_AUTH_ENDPOINT")
			postJson := []byte(`{"email":"` + ba.email + `","password":"` + ba.password + `"}`)

			req, err := http.NewRequest("POST", url, bytes.NewBuffer(postJson))
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				logrus.WithFields(logrus.Fields{"service": "checker", "error": err}).Warn("Couldn't sychronize checks")
				return nil, errors.New("error during bastion auth")
			} else {
				defer resp.Body.Close()
				body, _ := ioutil.ReadAll(resp.Body)

				var auth map[string]interface{}
				byt := []byte(body)
				if err := json.Unmarshal(byt, &auth); err != nil {
					logrus.WithFields(logrus.Fields{"service": "checker", "event": "auth response", "response": resp}).Error("Couldn't unmarshall auth json")
					return nil, errors.New("Couldn't unmarshall json")
				} else {
					newToken = &BastionAuthToken{Type: bearerToken, Token: auth["token"].(string), Endpoint: endpoint}
					ba.tokenMap[newToken.Type] = newToken
					logrus.WithFields(logrus.Fields{"service": "checker", "event": "auth response", "response": resp}).Info("We're probably authed")
				}
			}
		} else {
			logrus.WithFields(logrus.Fields{"service": "checker", "email": ba.email, "password": ba.password}).Error("Couldn't get one of: CUSTOMER_EMAIL, or CUSTOMER_PASSWORD")
			return nil, errors.New("Couldn't get one of: CUSTOMER_EMAIL, or CUSTOMER_PASSWORD")
		}

	case basicToken:
		ba.customerId = os.Getenv("CUSTOMER_ID")
		ba.email = os.Getenv("CUSTOMER_EMAIL")

		if len(ba.customerId) > 0 && len(ba.email) > 0 {
			postJson := `{"email":"` + ba.email + `","customer_id":"` + ba.customerId + `"}`
			newToken = &BastionAuthToken{Type: basicToken, Token: base64.StdEncoding.EncodeToString([]byte(postJson)), Endpoint: endpoint}
			ba.tokenMap[newToken.Type] = newToken
			logrus.WithFields(logrus.Fields{"service": "checker", "event": "created token", "token": newToken}).Info("generated basic auth token")
		} else {
			logrus.WithFields(logrus.Fields{"service": "checker", "email": ba.email, "customerId": ba.customerId}).Error("Couldn't get one of: CUSTOMER_ID, CUSTOMER_EMAIL")

			return nil, errors.New("Couldn't get one of: CUSTOMER_ID, CUSTOMER_EMAIL")
		}

	}
	return newToken, nil
}
