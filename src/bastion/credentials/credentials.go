package credentials

import (
		"time"
		"net/http"
		"fmt"
		"io/ioutil"
		"encoding/json"
)

type Credentials struct {
	Code 			string
	LastUpdated 	string
	Type 			string
	AccessKeyId		string
	SecretAccessKey string
	Token			string
	Expiration		string
}

func Start() <-chan Credentials {
	cc := make(chan Credentials, 1)
	ticks := time.Tick(1 * time.Hour)
	go func() {
		for {
			if !loop(cc, ticks) {
				return
			}
		}
	}()
	return cc
}

func loop(cc chan Credentials, ticks <-chan time.Time) bool {
	resp, err := http.Get("http://169.254.169.254/latest/meta-data/iam/security-credentials/opsee")
	if err != nil {
		fmt.Println("error getting ec2 metadata: ", err)
		return true
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error reading ec2 metadata: ", err)
		return true
	}
	var creds Credentials
	err = json.Unmarshal(body, &creds)
	if err != nil {
		fmt.Println("error parsing credentials: ", err)
		return true
	}
	cc <- creds
	<- ticks
	return true
}