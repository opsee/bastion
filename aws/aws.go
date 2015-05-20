package aws

import (
    "github.com/opsee/bastion/Godeps/_workspace/src/github.com/op/go-logging"
)

var (
    log = logging.MustGetLogger("aws")
    logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}")
)

func init() {
    logging.SetLevel(logging.DEBUG, "aws")
    logging.SetFormatter(logFormat)
}
