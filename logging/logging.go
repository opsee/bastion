package logging

import logs "github.com/op/go-logging"

const (
	loggerFormatString = "%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}"
)

var (
	loggerFormat = logs.MustStringFormatter(loggerFormatString)
	level        = logs.INFO
)

func GetLogger(module string) *logs.Logger {
	logs.SetLevel(level, module)
	logs.SetFormatter(loggerFormat)
	return logs.MustGetLogger(module)
}

func SetLevel(level logs.Level, module string) {
	logs.SetLevel(level, module)
}
