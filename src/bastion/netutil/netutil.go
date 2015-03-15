package netutil

import (
	"github.com/op/go-logging"
)

var (
	log       = logging.MustGetLogger("json-tcp")
	logFormat = logging.MustStringFormatter("%{time:2006-01-02T15:04:05.999999999Z07:00} %{level} [%{module}] %{message}")
)

func init() {
	logging.SetLevel(logging.INFO, "json-tcp")
	logging.SetFormatter(logFormat)
}
