package checker

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/opsee/basic/schema"
	"golang.org/x/net/context"
)

// SlateClient -- for clienting slates.
type SlateClient struct {
	slateUrl   string
	httpClient *http.Client
	// MaxRetries is the number of times the SlateClient will retry a failed
	// request. Default: 11
	MaxRetries int
}

type SlateRequest struct {
	Assertions []*schema.Assertion `json:"assertions"`
	Response   json.RawMessage     `json:"response"`
}

type SlateResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

func NewSlateClient(slateUrl string) *SlateClient {
	s := &SlateClient{
		slateUrl: slateUrl,
		httpClient: &http.Client{
			// This feels pretty generous, but assume that occasionally
			// shit will crash.
			Timeout: 30 * time.Second,
		},
		MaxRetries: 11,
	}

	return s
}

// CheckAssertions issues a request to Slate to determine if a check response
// is passing or failing.
func (s *SlateClient) CheckAssertions(ctx context.Context, check *schema.Check, checkResponse json.RawMessage) (bool, error) {
	var (
		body        []byte
		success     bool
		clientError error
		slateResp   SlateResponse
	)

	sr := &SlateRequest{
		Assertions: check.Assertions,
		Response:   checkResponse,
	}

	marshalledSlateRequest, err := json.Marshal(sr)
	if err != nil {
		log.WithError(err).Error("Couldn't marshal slate request")
	}

	bodyReader := bytes.NewReader(marshalledSlateRequest)
	req, err := http.NewRequest("POST", s.slateUrl, bodyReader)
	if err != nil {
		return success, err
	}

	for i := 0; i < s.MaxRetries; i++ {
		resp, err := s.httpClient.Do(req)
		if err != nil {
			clientError = err
			goto ERROR
		}

		defer resp.Body.Close()
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Info(body)
			clientError = err
			goto ERROR
		}
		log.WithFields(log.Fields{"response": string(body)}).Debug("Got Slate response.")

		err = json.Unmarshal(body, &slateResp)
		if err != nil {
			clientError = err
			goto ERROR
		}

		success = slateResp.Success
		break

	ERROR:
		if clientError != nil {
			log.WithError(clientError).Error("Issuing POST request to slate.")
			// Check to see if the context was cancelled/deadline was exceeded.
			if ctx.Err() != nil {
				return success, ctx.Err()
			}
			time.Sleep((1 << uint(i+1)) * time.Millisecond * 10)
			bodyReader.Seek(0, 0)
			continue
		}
	}

	return success, clientError
}
