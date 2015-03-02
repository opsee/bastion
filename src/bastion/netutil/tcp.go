package netutil

import "time"

import "io"
import "log"
import "net"
import "bufio"
import "strconv"
import "crypto/tls"
import "crypto/rand"
import "net/textproto"

// acceptor exhaustion strategy
const (
    RefuseConnection = iota
    IncreaseConnection
)

type Listener struct {
    AcceptorCount int
    MaxConnections int
    ListenPort  int
    Handler Handler
    ListenerChannel chan string
    OverflowStrategy int
    SslOptions   map[string]string
}

type Handler func(string, Connection)

type Connection struct {
    Conn  net.Conn
    Write chan []byte
    Quit chan bool
    Listener *Listener
}

func (c *Connection) Close() {
    c.Quit <- true
}

func (c *Connection) Send(out []byte) {
    c.Conn.Write(out)
}

func (l *Listener) StopTcpListener() {
    l.ListenerChannel <- "stop"
}

func InitNewTcpListener(acceptorCount int,
                        maxConnections int,
                        port int,
                        handler Handler,
                        listenerChannel chan string,
                        overflowStrategy int,
                        sslOptions map[string]string) *Listener {
    listener := &Listener{}
    listener.AcceptorCount = acceptorCount
    listener.MaxConnections = maxConnections
    listener.ListenPort = port
    listener.Handler = handler
    listener.ListenerChannel = listenerChannel
    listener.SslOptions = sslOptions
    listener.OverflowStrategy = overflowStrategy
    return listener
}

func StartNewTcpListener(listener *Listener) {
    var err error
    var ln net.Listener
    var cert tls.Certificate
    var connsCouter = 0

    if len(listener.SslOptions) == 0 {
        ln, err = net.Listen("tcp", ":" + strconv.Itoa(listener.ListenPort))
    } else {
        cert, err = tls.LoadX509KeyPair(listener.SslOptions["pem"], listener.SslOptions["key"])
        config := tls.Config{Certificates: []tls.Certificate{cert}}
        config.Rand = rand.Reader
        ln, err = tls.Listen("tcp", ":" + strconv.Itoa(listener.ListenPort), &config)
    }
    log.Print()
    if err != nil {
        log.Print("[Error] TCP listener didn't start: ", err)
        return
    }

    // acceptor -> listener
    connectionCounter := make(chan int)
    // listener -> acceptor
    closeConnection := make(chan bool)

    for accs := 0; accs < listener.AcceptorCount; accs++ {
        go acceptor(ln, connectionCounter, listener, closeConnection)
    }

    for {
        select {
        case msg := <-connectionCounter:
            if msg == -1 {
                connsCouter--
            } else if msg == 1 {
                connsCouter++
            } else {}
        case msg := <-listener.ListenerChannel:
            if msg == "stop" {
                closeConnection <- true
                close(listener.ListenerChannel)
                close(connectionCounter)
                ln.Close()
                return
            } else if msg == "getConnectionsCount" {
                listener.ListenerChannel <- strconv.Itoa(connsCouter)
            }
        default:
            time.Sleep(100 * time.Millisecond)
        }
    }
}

func acceptor(ln net.Listener, connectionCounter chan int, listener *Listener, closeChannel chan bool) {
    for {
        conn, err := ln.Accept()
        listener.ListenerChannel <- "getConnectionsCount"
        connsCount := <-listener.ListenerChannel
        conns, _ := strconv.Atoi(connsCount)
        if conns >= listener.MaxConnections {
            if listener.OverflowStrategy == RefuseConnection {
                conn.Close()
            } else if listener.OverflowStrategy == IncreaseConnection {
                listener.AcceptorCount += 128
                StartNewConnection(connectionCounter, conn, listener, err, closeChannel)
            }
        } else {
            StartNewConnection(connectionCounter, conn, listener, err, closeChannel)
        }
    }
}

func StartNewConnection(connectionCounter chan int, conn net.Conn, listener *Listener, err error, closeChannel chan bool) {
    if err != nil {
        log.Print("[Error] Tcp listener can't accept new connection: ")
    } else {
        connectionCounter <- 1
        go HandleNewConnection(connectionCounter, conn, listener, closeChannel)
    }
}

func HandleNewConnection(connectionCounter chan int, conn net.Conn, listener  *Listener, closeChannel chan bool) {
    newConnection := &Connection{conn, make(chan []byte), make(chan bool), listener}
    reader := textproto.NewReader(bufio.NewReader(conn))
    for {
        select {
        case msg := <-newConnection.Quit:
            if msg == true {
                connectionCounter <- -1
                conn.Close()
                return
            }
        case msg := <-closeChannel:
            if msg == true {
                connectionCounter <- -1
                conn.Close()
            }
            close(newConnection.Write)
            close(newConnection.Quit)
            return
        default:
        }
//        line, inputErr := textproto.NewReader(bufio.NewReader(conn)).ReadLine()
        line, inputErr := reader.ReadLine()
        if inputErr == io.EOF {
            connectionCounter <- -1
            conn.Close()
            return
        }
        go listener.Handler(line, *newConnection)
    }
}


/*  Sample main function and handler implementation for an echo server
func main() {
    done := make(chan bool)
    acceptorsCount := 2
    lc := make(chan string)
    strategy := RefuseConnection
    ssl := make(map[string]string)
    maxConnections := 50
    listener := InitNewTcpListener(acceptorsCount, maxConnections, 5666, handler, lc, strategy, ssl)
    go StartNewTcpListener(listener)
    <-done
}

func handler(input string, conn Connection) {
    conn.Send([]byte("Hello"))
}
*/
