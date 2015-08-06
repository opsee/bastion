package logging

import logs "github.com/op/go-logging"

const (
	loggerFormatString = "%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}"
)

var (
	loggerFormat            = logs.MustStringFormatter(loggerFormatString)
	level        logs.Level = logs.INFO
)

func GetLogger(module string) *logs.Logger {
	logs.SetLevel(level, module)
	logs.SetFormatter(loggerFormat)
	return logs.MustGetLogger(module)
}

func SetLevel(newLevel string, module string) error {
	var err error
	level, err = logs.LogLevel(newLevel)
	if err == nil {
		logs.SetLevel(level, module)
	}
	return err
}
