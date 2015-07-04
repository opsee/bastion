package main

import (
	"fmt"
	"github.com/op/go-logging"
	"github.com/opsee/bastion"
	//"github.com/opsee/bastion/netutil"
)

var (
	log       = logging.MustGetLogger("bastion")
	logFormat = logging.MustStringFormatter("%{color}%{time:15:04:05.000} [%{level:.8s}]: [%{module}] %{shortfunc} ▶ %{id:03x}%{color:reset} %{message}")
)

func main() {
	config := bastion.GetConfig()
	fmt.Println("config %s", config)
}
