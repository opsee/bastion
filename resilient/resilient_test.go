package resilient

import (
	"fmt"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/stretchr/testify/assert"
	nettest "github.com/opsee/bastion/testing/net"
	"os"
	"testing"
	"time"
)

func TestSuccessfulConnection(t *testing.T) {
	fmt.Println("test success")
	server := startServer(6590, t)
	conn, err := Start("127.0.0.1:6590",
		certPath("server/server-cert.pem"),
		certPath("client/creds/01-cert.pem"),
		certPath("client/creds/01-key.pem"))
	if err != nil {
		t.Fatalf("error in start client %v", err)
	}
	defer conn.Close()
	c := <-server.Startup
	fmt.Println("conn")
	c.Write([]byte{0, 0})
	fmt.Println("write")
	server.Close()
}

func TestUnsuccessfulConnection(t *testing.T) {
	fmt.Println("test unsuccessful")
	conn, err := Start("127.0.0.1:6590",
		certPath("server/server-cert.pem"),
		certPath("client/creds/01-cert.pem"),
		certPath("client/creds/01-key.pem"))
	if err != nil {
		t.Fatalf("error in start client %v", err)
	}
	defer conn.Close()
	time.Sleep(time.Millisecond * 100)
	assert.False(t, conn.IsConnected())
	fmt.Println("starting server")
	server := startServer(6590, t)
	fmt.Println("server started")
	<-server.Startup
	fmt.Println("startup achieved")
	time.Sleep(time.Millisecond * 10)
	assert.True(t, conn.WaitConnect())
	server.Close()
}

type tester struct {
	A string
	B int
	C bool
}

func TestConnectAndSend(t *testing.T) {
	server := startServer(6590, t)
	conn, err := Start("127.0.0.1:6590",
		certPath("server/server-cert.pem"),
		certPath("client/creds/01-cert.pem"),
		certPath("client/creds/01-key.pem"))
	if err != nil {
		t.Fatalf("error in start client %v", err)
	}
	defer conn.Close()
	<-server.Startup
	toBeSent := tester{"test", 15, true}
	conn.Send(toBeSent)
	str := "{\"A\":\"test\",\"B\":15,\"C\":true}"
	assert.Equal(t, []byte{0, 0x1c}, <-server.Recv)
	assert.Equal(t, []byte(str), <-server.Recv)
	server.Close()
}

func TestConnectAndRecv(t *testing.T) {
	server := startServer(6590, t)
	conn, err := Start("127.0.0.1:6590",
		certPath("server/server-cert.pem"),
		certPath("client/creds/01-cert.pem"),
		certPath("client/creds/01-key.pem"))
	if err != nil {
		t.Fatalf("error in start client %v", err)
	}
	defer conn.Close()
	c := <-server.Startup
	str := "{\"A\":\"test1\",\"B\":10,\"C\":false}"
	c.Write([]byte{0, byte(len(str))})
	c.Write([]byte(str))
	m := conn.Recv().(map[string]interface{})
	assert.Equal(t, "test1", m["A"])
	assert.Equal(t, 10, m["B"])
	assert.Equal(t, false, m["C"])
	server.Close()
}

func startServer(port int, t *testing.T) nettest.TestServer {
	config, err := loadConfig(certPath("client/client-cert.pem"),
		certPath("server/creds/01-cert.pem"),
		certPath("server/creds/01-key.pem"))
	if err != nil {
		t.Fatalf("error in load config %v", err)
	}
	return nettest.TlsServer(port, config, t)
}

func certPath(cert string) string {
	travis_build := os.Getenv("TRAVIS_BUILD_DIR")
	if travis_build != "" {
		return fmt.Sprintf("%s/certs/%s", travis_build, cert)
	}
	return fmt.Sprintf("%s/src/github.com/opsee/bastion/pkgdata/certs/%s", os.Getenv("GOPATH"), cert)
}
