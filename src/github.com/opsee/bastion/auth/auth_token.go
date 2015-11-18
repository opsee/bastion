package auth

import (
	"fmt"
	"strings"
	"time"
)

type BastionAuthTokenType string

const (
	BEARER_TOKEN  = BastionAuthTokenType("BEARER_TOKEN")
	BASIC_TOKEN   = BastionAuthTokenType("BASIC_TOKEN")
	UNKNOWN_TOKEN = BastionAuthTokenType("UNKNOWN_TOKEN")
)

func (tokentype BastionAuthTokenType) String() string {
	switch tokentype {
	case BEARER_TOKEN:
		return "BEARER_TOKEN"
	case BASIC_TOKEN:
		return "BASIC_TOKEN"
	}

	return "UKNOWN_TOKEN"
}

func (tokentype BastionAuthTokenType) Prefix() string {
	switch tokentype {
	case BEARER_TOKEN:
		return "Bearer"
	case BASIC_TOKEN:
		return "Basic"
	}

	return "UKNOWN_TOKEN"
}

func GetTokenTypeByString(tokentype string) (BastionAuthTokenType, error) {
	switch tokentype {
	case "BEARER_TOKEN":
		return BEARER_TOKEN, nil
	case "BASIC_TOKEN":
		return BASIC_TOKEN, nil
	}

	return UNKNOWN_TOKEN, fmt.Errorf("Unknown token type: %s", tokentype)
}

type BastionAuthToken struct {
	Type       BastionAuthTokenType
	Token      string
	Expiration int64
}

func (token *BastionAuthToken) AuthHeader() (string, string) {
	return "Authorization", strings.Join([]string{token.Type.Prefix(), token.Token}, " ")
}

func (token *BastionAuthToken) expired() bool {
	if token.Expiration < time.Now().Unix() {
		return true
	}
	return false
}
