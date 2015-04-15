package bastion

type Config struct {
    accessKeyId string // AWS Access Key Id
    secretKey   string // AWS Secret Key
    region      string // AWS Region Name
    opsee       string // Opsee home IP address
    caPath      string // path to CA
    certPath    string // path to TLS cert
    keyPath     string // path to cert privkey
    dataPath    string // path to event logfile for replay
    hostname    string // this machine's hostname
}

func (c *Config) AccessKeyId() string {
    return c.AccessKeyId()
}