package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type BastionAuthTokenRequest struct {
	TokenType        BastionAuthTokenType // defined in auth_token.go
	CustomerEmail    string
	CustomerPassword string
	CustomerID       string
	TargetEndpoint   string
	AuthEndpoint     string
}

type BastionAuthCache struct {
	Tokens map[string]*BastionAuthToken
}

func (cache *BastionAuthCache) generateToken(request *BastionAuthTokenRequest) (*BastionAuthToken, error) {
	switch request.TokenType {
	case BEARER_TOKEN:
		if request.CustomerEmail != "" && request.CustomerPassword != "" && request.AuthEndpoint != "" {
			postJson := []byte(`{"email":"` + request.CustomerEmail + `","password":"` + request.CustomerPassword + `"}`)

			req, err := http.NewRequest("POST", request.AuthEndpoint, bytes.NewBuffer(postJson))
			if err != nil {
				return nil, fmt.Errorf("Error during http request creation: %s", err.Error())
			}

			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("Error during bastion auth: %s", err.Error())
			}

			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)

			var auth map[string]interface{}
			if err := json.Unmarshal(body, &auth); err != nil {
				return nil, fmt.Errorf("Error unmarshalling json: %s", err.Error())
			} else {
				tokeStr, ok := auth["token"].(string)
				if !ok {
					return nil, fmt.Errorf("Unable to read returned auth token: %s", auth["token"])
				}

				newToken := &BastionAuthToken{
					Type:  request.TokenType,
					Token: tokeStr,
				}
				cache.Tokens[request.TargetEndpoint] = newToken
				return newToken, nil
			}
		} else {
			return nil, fmt.Errorf("BastionAuthTokenRequest missing one of: CustomerEmail: %s, CustomerPassword: %s, AuthEndpoint:%s, or TokenType:%s", request.CustomerEmail, request.CustomerPassword, request.AuthEndpoint, request.TokenType)
		}

	case BASIC_TOKEN:
		if request.CustomerID != "" && request.CustomerEmail != "" {
			postJson := `{"email":"` + request.CustomerEmail + `","customer_id":"` + request.CustomerID + `"}`
			newToken := &BastionAuthToken{
				Type:  request.TokenType,
				Token: base64.StdEncoding.EncodeToString([]byte(postJson)),
			}

			cache.Tokens[request.TargetEndpoint] = newToken
			return newToken, nil
		} else {
			return nil, fmt.Errorf("BastionAuthTokenRequest missing one of: CustomerEmail: %s, CustomerID: %s, AuthEndpoint:%s, or TokenType:%s", request.CustomerEmail, request.CustomerPassword, request.AuthEndpoint, request.TokenType)
		}
	}

	return nil, fmt.Errorf("Undefined Token Type: %s", request.TokenType)
}

func (cache *BastionAuthCache) GetToken(request *BastionAuthTokenRequest) (*BastionAuthToken, error) {
	// check for token expiry, return if we have a current token
	if token, ok := cache.Tokens[request.TargetEndpoint]; ok {
		if !token.expired() {
			return token, nil
		}
	}

	return cache.generateToken(request)
}
