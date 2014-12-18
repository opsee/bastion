package main

import (
		"fmt"
		"flag"
		"bastion/credentials"
		// "github.com/bmizerany/aws4"
)

// we must first retrieve our AWS API keys, which will either be in the instance metadata,
// or our command line options. Then we begin scanning the environment, first using the AWS
// API, and then actually trying to open TCP connections.

// In parallel we try and open a TLS connection back to the opsee API. We'll have been supplied
// a ca certificate, certificate and a secret key in pem format, either via the instance metadata 
// or on the command line.

var accessKeyId string
var secretKey string
var opsee string
var caPath string
var certPath string
var keyPath string

func init() {
	flag.StringVar(&accessKeyId, "access_key_id", "", "AWS access key ID.")
	flag.StringVar(&secretKey, "secret_key", "", "AWS secret key ID.")
	flag.StringVar(&opsee, "opsee", "localhost:8085", "Hostname and port to the Opsee server.")
	flag.StringVar(&caPath, "ca", "ca.pem", "Path to the CA certificate.")
	flag.StringVar(&certPath, "cert", "cert.pem", "Path to the certificate.")
	flag.StringVar(&keyPath, "key", "key.pem", "Path to the key file.")
}

func main() {
	flag.Parse()

	if accessKeyId == "" || secretKey == "" {
		cc := credentials.Start()
		creds := <- cc
		accessKeyId = creds.AccessKeyId
		secretKey = creds.SecretAccessKey
		fmt.Println("creds ", creds)
	}


}