package logging

import (
	"github.com/op/go-logging"
)

const (
	loggerFormatString = "%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}"
)

var (
	loggerFormat = logging.MustStringFormatter(loggerFormatString)
)

func GetLogger(module string) *logging.Logger {
	logging.SetLevel(logging.DEBUG, module)
	logging.SetFormatter(loggerFormat)
	return logging.MustGetLogger(module)
}
