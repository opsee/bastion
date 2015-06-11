package aws

import (
    "encoding/json"
    "io/ioutil"
    "net/http"
    "sync"
    "time"
)

type HttpClient interface {
    Get(url string) (resp *http.Response, err error)
}

type CredentialsProvider struct {
    creds            chan *Credentials
    oldCreds         *Credentials
    instanceId       *InstanceId
    ticks            <-chan time.Time
    client           HttpClient
    startupWaitGroup *sync.WaitGroup
}

type Credentials struct {
    AccessKeyId     string
    SecretAccessKey string
    Region          string
}

type metadataCredentials struct {
    Code            string
    LastUpdated     string
    Type            string
    AccessKeyId     string
    SecretAccessKey string
    Token           string
    Expiration      string
}

type InstanceId struct {
    InstanceId       string
    Architecture     string
    ImageId          string
    InstanceType     string
    KernelId         string
    RamdiskId        string
    Region           string
    Version          string
    PrivateIp        string
    AvailabilityZone string
}

func NewProvider(client HttpClient,
overrideAccessKeyId, overrideSecretAccessKey, overrideRegion string) *CredentialsProvider {
    cp := &CredentialsProvider{make(chan *Credentials),
        &Credentials{overrideAccessKeyId, overrideSecretAccessKey, overrideRegion},
        nil,
        time.Tick(1 * time.Hour),
        client, new(sync.WaitGroup)}
    cp.start(overrideAccessKeyId, overrideSecretAccessKey, overrideRegion)
    cp.startupWaitGroup.Wait()
    return cp
}

func (cp *CredentialsProvider) start(overrideAccessKeyId, overrideSecretAccessKey, overrideRegion string) {
    go func() {
        if overrideAccessKeyId != "" && overrideSecretAccessKey != "" && overrideRegion != "" {
            cp.creds <- &Credentials{overrideAccessKeyId, overrideSecretAccessKey, overrideRegion}
            return
        }
        cp.startupWaitGroup.Add(1)
        iid := cp.retrieveInstanceId()
        cp.startupWaitGroup.Done()
        if cp.loopOnce(iid, overrideAccessKeyId, overrideSecretAccessKey, overrideRegion) {
            cp.loop(iid, overrideAccessKeyId, overrideSecretAccessKey, overrideRegion)
        }
    }()
}

func (cp *CredentialsProvider) loop(iid *InstanceId, overrideAccessKeyId string, overrideSecretAccessKey string, overrideRegion string) (res bool) {
    for {
        res = cp.loopOnce(iid, overrideAccessKeyId, overrideSecretAccessKey, overrideRegion)
        if !res {
            break
        }
    }
    return
}

func (cp *CredentialsProvider) loopOnce(iid *InstanceId, overrideAccessKeyId string, overrideSecretAccessKey string, overrideRegion string) bool {
    metaCreds := cp.retrieveMetadataCreds()
    var accessKeyId, secretAccessKey string
    if metaCreds != nil {
        accessKeyId = metaCreds.AccessKeyId
        if overrideAccessKeyId != "" {
            accessKeyId = overrideAccessKeyId
        }
        secretAccessKey = metaCreds.SecretAccessKey
        if overrideSecretAccessKey != "" {
            secretAccessKey = overrideSecretAccessKey
        }
    }
    var region string = ""
    if iid != nil {
        region = iid.Region
    }
    if overrideRegion != "" {
        region = overrideRegion
    }
    if region == "" {
        log.Error("No metadata available and no region supplied on cmd line. Exiting.")
        return false
    }
    creds := &Credentials{accessKeyId, secretAccessKey, region}
    cp.creds <- creds
    <-cp.ticks
    return true
}

func (cp *CredentialsProvider) Credentials() *Credentials {
    select {
    case creds := <-cp.creds:
        cp.oldCreds = creds
        return creds
    default:
        return cp.oldCreds
    }
}

func (cp *CredentialsProvider) GetInstanceId() *InstanceId {
    return cp.instanceId
}

func (cp *CredentialsProvider) retrieveInstanceId() *InstanceId {
    if resp, err := cp.client.Get("http://169.254.169.254/latest/dynamic/instance-identity/document"); err == nil {
		defer resp.Body.Close()
		if body, err := ioutil.ReadAll(resp.Body); err == nil {
			var iid InstanceId
			if err = json.Unmarshal(body, &iid); err == nil {
				cp.instanceId = &iid
				return &iid
			}
		}
	}
	return nil

}

func (cp *CredentialsProvider) retrieveMetadataCreds() *metadataCredentials {
    var resp *http.Response = nil
    var err error
    var body []byte
    if resp, err = cp.client.Get("http://169.254.169.254/latest/meta-data/iam/security-credentials/opsee"); err != nil {
        log.Error("error getting ec2 metadata:", err)
        return nil
    }
    if resp == nil {
        return nil
    }
    defer resp.Body.Close()
    if body, err = ioutil.ReadAll(resp.Body); err != nil {
        log.Error("error reading ec2 metadata:", err)
        return nil
    }
    var metaCreds metadataCredentials
    if err = json.Unmarshal(body, &metaCreds); err != nil {
        log.Error("error parsing credentials:", err)
        return nil
    }
    return &metaCreds
}
