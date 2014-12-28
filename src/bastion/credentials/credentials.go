package credentials

import (
		"time"
		"net/http"
		"fmt"
		"io/ioutil"
		"encoding/json"
)

type CredentialsProvider struct {
	creds 			chan *Credentials
	oldCreds		*Credentials
	ichan			chan *InstanceId
	ticks		 	<-chan time.Time
}

type Credentials struct {
	AccessKeyId		string
	SecretAccessKey string
	Region			string
}

type metadataCredentials struct {
	Code 			string
	LastUpdated 	string
	Type 			string
	AccessKeyId		string
	SecretAccessKey string
	Token			string
	Expiration		string
}

type InstanceId struct {
	InstanceId 			string
	Architecture		string
	ImageId				string
	InstanceType		string
	KernelId			string
	RamdiskId			string
	Region 				string
	Version 			string
	PrivateIp 			string
	AvailabilityZone 	string
}

func New(overrideAccessKeyId string, overrideSecretAccessKey string, overrideRegion string) *CredentialsProvider {
	cp := CredentialsProvider {make(chan *Credentials),
		&Credentials{overrideAccessKeyId,overrideSecretAccessKey,overrideRegion},
		make(chan *InstanceId),
		time.Tick(1 * time.Hour)}
	cp.start(overrideAccessKeyId, overrideSecretAccessKey, overrideRegion)
	return &cp
}

func (cp *CredentialsProvider) start(overrideAccessKeyId string, overrideSecretAccessKey string, overrideRegion string) {
	go func() {
		if overrideAccessKeyId != "" && overrideSecretAccessKey != "" && overrideRegion != "" {
			cp.creds <- &Credentials{overrideAccessKeyId, overrideSecretAccessKey, overrideRegion}
			return
		}
		iid := cp.retrieveInstanceId()
		if iid != nil {
			cp.ichan <- iid
		}
		for {
			if !cp.loop(iid, overrideAccessKeyId, overrideSecretAccessKey, overrideRegion) {
				return
			}
		}
	}()
}

func (cp *CredentialsProvider) loop(iid *InstanceId, overrideAccessKeyId string, overrideSecretAccessKey string, overrideRegion string) bool {
	metaCreds := cp.retrieveMetadataCreds()
	accessKeyId := metaCreds.AccessKeyId
	if overrideAccessKeyId != "" {
		accessKeyId = overrideAccessKeyId
	}
	secretAccessKey := metaCreds.SecretAccessKey
	if overrideSecretAccessKey != "" {
		secretAccessKey = overrideSecretAccessKey
	}
	var region string = ""
	if iid != nil {
		region = iid.Region
	}
	if overrideRegion != "" {
		region = overrideRegion
	}
	if region == "" {
		fmt.Println("No metadata available and no region supplied on cmd line. Exiting.")
		return false
	}
	creds := &Credentials{accessKeyId, secretAccessKey, region}
	cp.creds <- creds
	<- cp.ticks
	return true
}

func (cp * CredentialsProvider) GetCredentials() *Credentials {
	select {
	case creds := <- cp.creds:
		cp.oldCreds = creds
		return creds
	default:
		return cp.oldCreds
	}
}

func (cp *CredentialsProvider) retrieveInstanceId() *InstanceId {
	resp,err := http.Get("http://169.254.169.254/latest/dynamic/instance-identity/document")
	if err != nil {
		fmt.Println("error getting ec2 instance id:", err)
		return nil
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error reading ec2 metadata:", err)
		return nil
	}
	var iid InstanceId
	err = json.Unmarshal(body, &iid)
	if err != nil {
		fmt.Println("error parsing instanceid:", err)
		return nil
	}
	cp.ichan <- &iid
	return &iid
}

func (cp *CredentialsProvider) retrieveMetadataCreds() *metadataCredentials {
	resp, err := http.Get("http://169.254.169.254/latest/meta-data/iam/security-credentials/opsee")
	if err != nil {
		fmt.Println("error getting ec2 metadata:", err)
		return nil
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error reading ec2 metadata:", err)
		return nil
	}
	var metaCreds metadataCredentials
	err = json.Unmarshal(body, &metaCreds)
	if err != nil {
		fmt.Println("error parsing credentials:", err)
		return nil
	}
	return &metaCreds
}