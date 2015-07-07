package messaging

import "github.com/opsee/bastion/logging"

// TODO: Migrate shared code and constantize magic strings/numbers here.

const (
	nsqdURL = "http://nsqd:4150"
)

var (
	logger = logging.GetLogger("messaging")
)

func getNsqdURL() string {
	return nsqdURL
}
