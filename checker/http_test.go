package checker

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/basic/schema"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

// don't follow redirects for 301s, make sure there is a response object with
// a Location header.
func TestRedirectResponse(t *testing.T) {
	ctx := context.Background()
	ts := httptest.NewServer(http.RedirectHandler("http://redirected/", 301))
	defer ts.Close()

	requestMaker := &HTTPRequest{Method: "GET", URL: ts.URL, Body: ""}
	resp := <-requestMaker.Do(ctx)
	if resp == nil {
		log.Fatal("TestRedirectResponse: Got nil response from http worker")
	}

	httpResponse, _ := resp.Response.(*schema.HttpResponse)

	assert.EqualValues(t, 301, httpResponse.Code, "response code should contain the redirect code")
	location := ""

	assert.NotEmpty(t, httpResponse.Headers, "response should have headers")

	for _, header := range httpResponse.Headers {
		if strings.ToLower(header.Name) == "location" {
			location = header.Values[0]
		}
	}

	assert.Equal(t, "http://redirected/", location, "redirect response should container correct location header")
}

// case where http server returns no response body
func TestResponseEmpty(t *testing.T) {
	ctx := context.Background()
	testResponse := ""

	log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestEmptyResponse", "action": "starting server"}).Info("starting server for test with no response body")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, testResponse)
	}))
	defer ts.Close()

	requestMaker := &HTTPRequest{Method: "GET", URL: ts.URL, Body: ""}
	resp := <-requestMaker.Do(ctx)
	err := resp.Error
	if err != nil {
		log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestEmptyResponse", "Error": "request error"}).Fatal(err)
	}

	if resp, ok := resp.Response.(*schema.HttpResponse); ok {
		log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestEmptyResponse", "response": resp.Body}).Info("received response")
		assert.Equal(t, testResponse, resp.Body, "response must match predefined test response (the empty string)")
	} else {
		log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestEmptyResponse", "response": resp.Body, "Error": "no response body"}).Fatal(err)
	}
}

// case where http server returns response body smaller than 4096 bytes
func TestResponseNormal(t *testing.T) {
	ctx := context.Background()
	testResponse, err := GenerateRandomString(2948)
	if err != nil {
		log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestResponseNormal", "Error": "error generating random response"}).Fatal(err)
	}

	log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestResponseNormal", "action": "starting server"}).Info("starting server for test with no response body")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, testResponse)
	}))
	defer ts.Close()

	requestMaker := &HTTPRequest{Method: "GET", URL: ts.URL, Body: ""}
	resp := <-requestMaker.Do(ctx)
	err = resp.Error
	if err != nil {
		log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestResponseNormal", "Error": "request error"}).Fatal(err)
	}

	if resp, ok := resp.Response.(*schema.HttpResponse); ok {
		log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TesResponseNormal", "response": resp.Body}).Info("received response")
		assert.Equal(t, testResponse, resp.Body, "response must match predefined test response")
	} else {
		log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestResponseNormal", "response": resp.Body, "Error": "no response body"}).Fatal(err)
	}
}

// case where http server returns response larger than 4096 bytes
func TestResponseTruncate(t *testing.T) {
	ctx := context.Background()
	testResponse, err := GenerateRandomString(MaxContentLength + 500)
	if err != nil {
		log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestResponseTruncate", "Error": "error generating random response"}).Fatal(err)
	}

	log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestResponseTruncate", "action": "starting server"}).Info("starting server for test with no response body")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, testResponse)
	}))
	defer ts.Close()

	requestMaker := &HTTPRequest{Method: "GET", URL: ts.URL, Body: ""}
	resp := <-requestMaker.Do(ctx)
	err = resp.Error
	if err != nil {
		log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestResponseTruncate", "Error": "request error"}).Fatal(err)
	}

	if resp, ok := resp.Response.(*schema.HttpResponse); ok {
		log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestResponseTruncate", "response": resp.Body}).Info("received response")
		assert.Equal(t, MaxContentLength, len(resp.Body), "body should be trucated to MaxContentLength bytes")
	} else {
		log.WithFields(log.Fields{"test unit": "checker/http.go", "test": "TestResponseTruncate", "response": resp.Body, "Error": "no response body"}).Fatal(err)
	}
}

// https://elithrar.github.io/article/generating-secure-random-numbers-crypto-rand/
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

func GenerateRandomString(s int) (string, error) {
	b, err := GenerateRandomBytes(s)
	return base64.URLEncoding.EncodeToString(b), err
}
