package aws

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"github.com/opsee/bastion/netutil"
)

const MetadataURL = "http://169.254.169.254/latest/dynamic/instance-identity/document"

type HttpClient interface {
	Get(url string) (resp *http.Response, err error)
}

type InstanceMeta struct {
	InstanceId string
	Architecture string
	ImageId string
	InstanceType string
	KernelId string
	RamdiskId string
	Region string
	Version string
	PrivateIp string
	AvailabilityZone string
}

type MetadataProvider struct {
	client HttpClient
	metadata *InstanceMeta
}

func (this MetadataProvider) Get() *InstanceMeta {
	if this.metadata != nil {
		return this.metadata
	}
	httpClient := this.client
	if httpClient == nil {
		httpClient = &http.Client{}
	}
		
	backoff := netutil.NewBackoffRetrier(func() (interface{}, error) {
		resp, err := this.client.Get(MetadataURL)
		if err != nil {
			log.Error("error getting ec2 instance metadata:", err)
			return nil, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error("error reading ec2 metadata:", err)
			return nil, err
		}
		meta := &InstanceMeta{}
		err = json.Unmarshal(body, meta)
		if err != nil {
			log.Error("error parsing instance metadata:", err)
			return nil, err
		}
		return meta, nil
	})
	
	err := backoff.Run()
	if err != nil {
		log.Error("backoff failed:", err)
		return nil
	}
	this.metadata = backoff.Result().(*InstanceMeta)
	return this.metadata
}
