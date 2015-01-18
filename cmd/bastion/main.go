package main

import (
		"fmt"
		"os"
		"io/ioutil"
		"flag"
		"time"
		"net/http"
		"bastion/credentials"
		// "bastion/ec2"
		// "bastion/resilient"
		"encoding/json"
		"github.com/amir/raidman"
)

// we must first retrieve our AWS API keys, which will either be in the instance metadata,
// or our command line options. Then we begin scanning the environment, first using the AWS
// API, and then actually trying to open TCP connections.

// In parallel we try and open a TLS connection back to the opsee API. We'll have been supplied
// a ca certificate, certificate and a secret key in pem format, either via the instance metadata 
// or on the command line.

var accessKeyId string
var secretKey string
var region string
var opsee string
var caPath string
var certPath string
var keyPath string
var dataPath string
var hostname string

func init() {
	flag.StringVar(&accessKeyId, "access_key_id", "", "AWS access key ID.")
	flag.StringVar(&secretKey, "secret_key", "", "AWS secret key ID.")
	flag.StringVar(&region, "region", "", "AWS Region.")
	flag.StringVar(&opsee, "opsee", "localhost:8085", "Hostname and port to the Opsee server.")
	flag.StringVar(&caPath, "ca", "ca.pem", "Path to the CA certificate.")
	flag.StringVar(&certPath, "cert", "cert.pem", "Path to the certificate.")
	flag.StringVar(&keyPath, "key", "key.pem", "Path to the key file.")
	flag.StringVar(&dataPath, "data", "", "Data path.")
	flag.StringVar(&hostname, "hostname", "", "Hostname override.")
}

func main() {
	flag.Parse()
	httpClient := &http.Client{}
	credProvider := credentials.NewProvider(httpClient, accessKeyId, secretKey, region)
	// c := ec2.Start(credProvider)
	c, err := raidman.Dial("tcp", opsee)
	if err != nil { //we'll need retry logic here but for right now I just need the frickin build to go
		fmt.Println("err",err)
		time.Sleep(30 * time.Second)
		return
	}

	if hostname == "" {
		hostname = credProvider.GetInstanceId().InstanceId
	}

	tick := time.Tick(time.Second * 10)

	go func() {
		if dataPath != "" {
			file, err := os.Open(dataPath)
			if err != nil {
				panic(err)
			}
			bytes, err := ioutil.ReadAll(file)
			if err != nil {
				panic(err)
			}
			events := []raidman.Event{}
			err = json.Unmarshal(bytes, &events)
			if err != nil {
				panic(err)
			}
			discTick := time.Tick(time.Second * 5)
			for _,event := range events {
				<- discTick
				fmt.Println(event)
				c.Send(&event)
			}
		}
	}()

	for {
		event := &raidman.Event{
			State: "connected",
			Host: hostname,
			Service: "bastion",
			Ttl: 10}
		fmt.Println(event)
		c.Send(event)
		<- tick
	}
}