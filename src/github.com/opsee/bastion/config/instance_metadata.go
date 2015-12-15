package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/opsee/bastion/netutil"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	MetadataURL  = "http://169.254.169.254/latest/dynamic/instance-identity/document"
	InterfaceURL = "http://169.254.169.254/latest/meta-data/network/interfaces/macs/"
)

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
	Timestamp        int64
	VPCID            string
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
			meta.Timestamp = time.Now().Unix()
			err = json.Unmarshal(metad, meta)

			if err == nil {
				return &MetadataProvider{
					client:   client,
					metadata: meta,
				}
			}
		} else {
			println(err.Error())
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
	this.metadata.Timestamp = time.Now().Unix()
	this.metadata.Hostname = netutil.GetHostnameDefault("")
	// since vpc ID is optional (for classic environments), we don't care if this fails
	this.metadata.VPCID = this.getVPC()

	return this.metadata
}

func (this MetadataProvider) getVPC() string {
	backoff := netutil.NewBackoffRetrier(func() (interface{}, error) {
		ifs, err := this.client.Get(InterfaceURL)
		if err != nil {
			logger.Error("error getting ec2 interface data:", err)
			return "", err
		}

		defer ifs.Body.Close()
		ifsbody, err := ioutil.ReadAll(ifs.Body)
		if err != nil {
			logger.Error("error reading ec2 interfaces:", err)
			return "", err
		}

		macs := bytes.Split(ifsbody, []byte("\n"))
		if len(macs) == 0 {
			logger.Error("error reading ec2 interfaces: none found")
			return "", fmt.Errorf("no interfaces found")
		}

		vpcres, err := this.client.Get(fmt.Sprintf("%s%svpc-id", InterfaceURL, string(macs[0])))
		if err != nil {
			logger.Error("error getting ec2 vpc id:", err)
			return "", err
		}

		defer vpcres.Body.Close()
		vpc, err := ioutil.ReadAll(vpcres.Body)
		if err != nil {
			logger.Error("error reading ec2 vpc id:", err)
			return "", err
		}

		return string(bytes.TrimSpace(vpc)), nil
	})

	err := backoff.Run()
	if err != nil {
		logger.Error("backoff failed:", err)
		return ""
	}

	res, ok := backoff.Result().(string)
	if !ok {
		return ""
	}

	return res
}
