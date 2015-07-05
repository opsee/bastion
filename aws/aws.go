package aws

import (
	"github.com/op/go-logging"
)

var (
	logger       = logging.MustGetLogger("aws")
	loggerFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}")
)

func init() {
	logging.SetLevel(logging.DEBUG, "aws")
	logging.SetFormatter(loggerFormat)
}
