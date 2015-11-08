package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/nsqio/go-nsq"
	"github.com/opsee/bastion/checker"
	"github.com/opsee/bastion/config"
	"github.com/opsee/bastion/heart"
	"github.com/opsee/portmapper"
	"github.com/parnurzeal/gorequest"
	"os"
	"strings"
)

const (
	moduleName = "checker"
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
			request := gorequest.New()
			postJson := `{"email":"` + ba.email + `","password":"` + ba.password + `"}`

			resp, body, errs := request.Post(os.Getenv("BASTION_AUTH_ENDPOINT")).Set("Accept-Encoding", "gzip, deflate").Set("Accept-Language", "en-US,en;q=0.8").Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_0) AppleWebKit/537.36(KHTML, like Gecko) Chrome/45.0.2454.101 Safari/537.36").Set("Content-Type", "application/json").Send(postJson).End()

			if len(errs) > 0 {
				log.WithFields(log.Fields{"service": "checker", "error": errs}).Warn("Couldn't sychronize checks")
				return nil, errors.New("error during bastion auth")
			} else {
				var auth map[string]interface{}
				byt := []byte(body)
				if err := json.Unmarshal(byt, &auth); err != nil {
					log.WithFields(log.Fields{"service": "checker", "event": "auth response", "response": resp}).Error("Couldn't unmarshall auth json")
					return nil, errors.New("Couldn't unmarshall json")
				} else {
					newToken = &BastionAuthToken{Type: bearerToken, Token: auth["token"].(string), Endpoint: endpoint}
					ba.tokenMap[newToken.Type] = newToken
					log.WithFields(log.Fields{"service": "checker", "event": "auth response", "response": resp}).Info("We're probably authed")
				}
			}
		} else {
			log.WithFields(log.Fields{"service": "checker", "email": ba.email, "password": ba.password}).Error("Couldn't get one of: CUSTOMER_EMAIL, or CUSTOMER_PASSWORD")
			return nil, errors.New("Couldn't get one of: CUSTOMER_EMAIL, or CUSTOMER_PASSWORD")
		}

	case basicToken:
		ba.customerId = os.Getenv("CUSTOMER_ID")
		ba.email = os.Getenv("CUSTOMER_EMAIL")

		if len(ba.customerId) > 0 && len(ba.email) > 0 {
			postJson := `{"email":"` + ba.email + `","customer_id":"` + ba.customerId + `"}`
			newToken = &BastionAuthToken{Type: basicToken, Token: base64.StdEncoding.EncodeToString([]byte(postJson)), Endpoint: endpoint}
			ba.tokenMap[newToken.Type] = newToken
			log.WithFields(log.Fields{"service": "checker", "event": "created token", "token": newToken}).Info("generated basic auth token")
		} else {
			log.WithFields(log.Fields{"service": "checker", "email": ba.email, "customerId": ba.customerId}).Error("Couldn't get one of: CUSTOMER_ID, CUSTOMER_EMAIL")

			return nil, errors.New("Couldn't get one of: CUSTOMER_ID, CUSTOMER_EMAIL")
		}

	}
	return newToken, nil
}

// XXX Fix when we add new check types
type Check struct {
	Id        string
	Interval  int32
	Target    *checker.Target
	LastRun   *checker.Timestamp
	CheckSpec *checker.Any
}

func getChecks() ([]Check, error) {
	ba := &BastionAuthCache{tokenMap: make(map[BastionAuthTokenType]*BastionAuthToken)}
	var checks []Check

	authType := ba.resolveAuthType(os.Getenv("BASTION_AUTH_TYPE"))
	endpoint := os.Getenv("BARTNET_ENDPOINT") + "/checks"
	if token, err := ba.getToken(authType, endpoint); err != nil || token == nil {
		log.WithFields(log.Fields{"service": "checker", "Error": err.Error()}).Fatal("Error initializing BastionAuth")
		return nil, err
	} else {
		log.WithFields(log.Fields{"service": "checker", "Auth header:": "Authorization: " + (string)(token.Type) + " " + (string)(token.Token)}).Info("Synchronizing checks")

		request := gorequest.New()
		resp, body, errs := request.Get(token.Endpoint).Set("Accept", "*/*").Set("User-Ageng", "dan-user").Set("Cache-Control", "no-cache").Set("Authorization", (string)(token.Type)+" "+(string)(token.Token)).End()

		if len(errs) > 0 {
			log.WithFields(log.Fields{"service": "checker", "error": errs, "response": resp}).Warn("Couldn't sychronize checks")
			return nil, err
		} else {
			log.WithFields(log.Fields{"service": "checker", "event": "got checks", "body": body}).Info("Recieved some checks")

			if err := json.Unmarshal([]byte(body), &checks); err != nil {
				return nil, err
			}
		}
	}

	return checks, nil
}

func main() {
	var err error

	runnerConfig := &checker.NSQRunnerConfig{}
	flag.StringVar(&runnerConfig.ConsumerQueueName, "results", "results", "Result queue name.")
	flag.StringVar(&runnerConfig.ProducerQueueName, "requests", "runner", "Requests queue name.")
	flag.StringVar(&runnerConfig.ConsumerChannelName, "channel", "runner", "Consumer channel name.")
	flag.IntVar(&runnerConfig.MaxHandlers, "max_checks", 10, "Maximum concurrently executing checks.")
	runnerConfig.NSQDHost = os.Getenv("NSQD_HOST")
	runnerConfig.CustomerID = os.Getenv("CUSTOMER_ID")

	config := config.GetConfig()
	log.WithFields(log.Fields{"service": "checker", "loglevel": config.LogLevel}).Info("Starting %s...", moduleName)

	checks := checker.NewChecker()
	runner, err := checker.NewRemoteRunner(runnerConfig)
	if err != nil {
		log.WithFields(log.Fields{"service": "checker", "Error": err.Error()}).Fatal("Error initializing runner runner.")
	}
	checks.Runner = runner
	scheduler := checker.NewScheduler()
	checks.Scheduler = scheduler

	producer, err := nsq.NewProducer(os.Getenv("NSQD_HOST"), nsq.NewConfig())

	if err != nil {
		log.WithFields(log.Fields{"service": "checker", "Error": err.Error()}).Fatal("Error creating NSQD producer.")
	}

	scheduler.Producer = producer

	// synchronize checks, if possible
	checksToSync, err := getChecks()

	if err != nil {
		log.WithFields(log.Fields{"service": "checker", "Error": err.Error()}).Fatal("Error creating NSQD producer.")
	} else {
		for c := 0; c < len(checksToSync); c++ {
			log.Info(checksToSync[c].Id)
			checkerCheck := &checker.Check{Id: checksToSync[c].Id, Interval: checksToSync[c].Interval, Target: checksToSync[c].Target, CheckSpec: checksToSync[c].CheckSpec}
			log.Info(checkerCheck)
			scheduler.CreateCheck(checkerCheck)
		}
	}

	defer checks.Stop()

	checks.Port = 4000
	if err = checks.Start(); err != nil {
		log.WithFields(log.Fields{"service": "checker"}).Error(err.Error())
		panic(err)
	}

	heart, err := heart.NewHeart(moduleName)
	if err != nil {
		log.WithFields(log.Fields{"service": "checker"}).Error(err.Error())
		panic(err)
	}

	portmapper.EtcdHost = os.Getenv("ETCD_HOST")
	portmapper.Register(moduleName, checks.Port)
	defer portmapper.Unregister(moduleName, checks.Port)

	err = <-heart.Beat()

	if err != nil {
		log.WithFields(log.Fields{"service": "checker"}).Error(err.Error())
		panic(err)
	}
}
