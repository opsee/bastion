package main
import (
    "bastion/netutil"
    "github.com/op/go-logging"
)

var (
    log       = logging.MustGetLogger("bastion.json-tcp")
    logFormat = logging.MustStringFormatter("%{time:2006-01-02T15:04:05.999999999Z07:00} %{level} [%{module}] %{message}")
)

func init() {
    logging.SetLevel(logging.INFO, "json-tcp")
    logging.SetFormatter(logFormat)
}

type Server struct{}

func (this *Server) SslOptions() netutil.SslOptions {
    return nil
}

func (this *Server) ConnectionMade(connection *netutil.Connection) bool {
    return true
}

func (this *Server) ConnectionLost(connection *netutil.Connection, err error) {
    log.Error("Connection lost: %v", err)
}

func (this *Server) RequestReceived(connection *netutil.Connection, request *netutil.ServerRequest) (reply *netutil.Reply, keepGoing bool) {
    isShutdown  := request.Command == "shutdown"
    keepGoing = !isShutdown
    if isShutdown {
        if err := connection.Server().Close(); err != nil {
            log.Notice("shutdown")
            reply = nil
        }
    }
    reply = netutil.NewReply(request)
    return
}


func main() {
    if server,err := netutil.ListenTCP(":5666", &Server{});  err == nil {
        server.Join()
    } else {
        log.Fatal("listwen %s", err.Error())
    }
}
