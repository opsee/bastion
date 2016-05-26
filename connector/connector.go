package connector

import (
	"fmt"
	"io"

	"github.com/opsee/basic/schema"
	"github.com/opsee/grinder"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type TestCheckRequestHandler func(*TestCheckRequest) *TestCheckResponse
type CheckRequestHandler func(*Check) *CheckResult

type Connector struct {
	gClient     *grinder.Client
	tcrHandlers []TestCheckRequestHandler
	crHandlers  []CheckRequestHandlers
}

func New(addr string) (*Connector, error) {
	conn, err := grpc.Dial(addr)
	if err != nil {
		return nil, err
	}

	client := grinder.NewClient(conn)

	connector := &Connector{
		gClient:     client,
		tcrHandlers: []TestCheckRequestHandler{},
		crHandlers:  []CheckRequestHandlers{},
	}

	err := connector.start()
	if err != nil {
		return nil, err
	}

	return connector
}

func (c *Connector) startHandlers(s grpc.Stream, hdnlrType string) {
	for {
		testCheckRequest, err := stream.Recv()
		if err == io.EOF {
			// TODO(greg): We should have a state machine maybe that handles
			// reconnects and backoff, but for now, just crash.
			log.Fatalf("TestCheckChannel closed by server.")
		}

		if err != nil {
			// TODO(greg): We may want to consider dying if we receive
			// too many consecutive errors (could be a protocol mismatch?).
			log.Errorf("Failed to receive a TestCheckRequest: %v", err)
		}

		// TODO(greg): use a github.com/grepory/scheduler to dispatch
		for _, h := range c.getHandlers(hdnlrType) {
			result := h(testCheckRequest)
			if result != nil {
				if err := stream.Send(result); err != nil {
					log.Errorf("Failed to send TestCheckResponse: %v", err)
				}
			}
		}
	}
}

func (c *Connector) start() error {
	// TODO(greg): Put auth material, etc in the context.
	tcc, err := c.gClient.TestCheck(context.Background())
	if err != nil {
		log.Errorf("Error opening TestCheckChannel: %v", err)
		return err
	}
	go c.startHandlers(tcc, "TestCheckRequest")

	cc, err := c.gClient.Check(context.Background())
	if err != nil {
		log.Errorf("Error opening CheckChannel: %v", err)
		return err
	}
	go c.startHandlers(cc, "CheckRequest")
}

func (c *Connector) getHandlers(t string) ([]func(interface{}) interface{}, error) {
	// You can't convince me this isn't easier than reflection.
	switch t {
	case "TestCheckRequest":
		return c.tcrHandlers
	case "CheckRequest":
		return c.crHandlers
	default:
		return fmt.Errorf("Unknown handler type: %s", t)
	}
}

func (c *Connector) AddTestCheckRequestHandler(handler TestCheckRequestHandler) {
	c.tcrHandlers = append(c.tcrHandlers, handler)
}

func (c *Connector) AddCheckRequestHandler(handler CheckRequestHandler) {
	c.crHandlers = append(c.crHandlers, handler)
}
