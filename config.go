package bastion
import "github.com/opsee/bastion/Godeps/_workspace/src/github.com/op/go-logging"

var (
    log       = logging.MustGetLogger("bastion.json-tcp")
    logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}")
)

func init() {
    logging.SetLevel(logging.DEBUG, "bastion.json-tcp")
    logging.SetFormatter(logFormat)
}


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