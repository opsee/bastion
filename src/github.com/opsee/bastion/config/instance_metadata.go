package config

import (
	"encoding/json"
	"github.com/opsee/bastion/netutil"
	"io/ioutil"
	"net/http"
)

const MetadataURL = "http://169.254.169.254/latest/dynamic/instance-identity/document"

type HttpClient interface {
	Get(url string) (resp *http.Response, err error)
}

type InstanceMeta struct {
	InstanceId       string
	Architecture     string
	ImageId          string
	InstanceType     string
	Hostname         string
	KernelId         string
	RamdiskId        string
	Region           string
	Version          string
	PrivateIp        string
	AvailabilityZone string
}

type MetadataProvider struct {
	client   HttpClient
	metadata *InstanceMeta
}

func NewMetadataProvider(client HttpClient, config *Config) *MetadataProvider {
	if config != nil && config.MDFile != "" {
		metad, err := ioutil.ReadFile(config.MDFile)
		if err == nil {
			meta := &InstanceMeta{}
			err = json.Unmarshal(metad, meta)
			if err == nil {
				return &MetadataProvider{
					client:   client,
					metadata: meta,
				}
			}
		}
	}
	return &MetadataProvider{
		client: client,
	}
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
			logger.Error("error getting ec2 instance metadata:", err)
			return nil, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logger.Error("error reading ec2 metadata:", err)
			return nil, err
		}
		meta := &InstanceMeta{}
		err = json.Unmarshal(body, meta)
		if err != nil {
			logger.Error("error parsing instance metadata:", err)
			return nil, err
		}
		return meta, nil
	})

	err := backoff.Run()
	if err != nil {
		logger.Error("backoff failed:", err)
		return nil
	}
	this.metadata = backoff.Result().(*InstanceMeta)
	this.metadata.Hostname = netutil.GetHostnameDefault("")
	return this.metadata
}
