package ec2query_test

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"github.com/awslabs/aws-sdk-go/internal/util"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/aws"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/internal/protocol/ec2query"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/internal/protocol/xml/xmlutil"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/awslabs/aws-sdk-go/internal/signer/v4"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"
)

var _ bytes.Buffer // always import bytes
var _ http.Request
var _ json.Marshaler
var _ time.Time
var _ xmlutil.XMLNode
var _ xml.Attr
var _ = ioutil.Discard
var _ = util.Trim("")
var _ = url.Values{}

// InputService1ProtocolTest is a client for InputService1ProtocolTest.
type InputService1ProtocolTest struct {
	*aws.Service
}

// New returns a new InputService1ProtocolTest client.
func NewInputService1ProtocolTest(config *aws.Config) *InputService1ProtocolTest {
	if config == nil {
		config = &aws.Config{}
	}

	service := &aws.Service{
		Config:      aws.DefaultConfig.Merge(config),
		ServiceName: "inputservice1protocoltest",
		APIVersion:  "2014-01-01",
	}
	service.Initialize()

	// Handlers
	service.Handlers.Sign.PushBack(v4.Sign)
	service.Handlers.Build.PushBack(ec2query.Build)
	service.Handlers.Unmarshal.PushBack(ec2query.Unmarshal)
	service.Handlers.UnmarshalMeta.PushBack(ec2query.UnmarshalMeta)
	service.Handlers.UnmarshalError.PushBack(ec2query.UnmarshalError)

	return &InputService1ProtocolTest{service}
}

// InputService1TestCaseOperation1Request generates a request for the InputService1TestCaseOperation1 operation.
func (c *InputService1ProtocolTest) InputService1TestCaseOperation1Request(input *InputService1TestShapeInputShape) (req *aws.Request, output *InputService1TestShapeInputService1TestCaseOperation1Output) {
	if opInputService1TestCaseOperation1 == nil {
		opInputService1TestCaseOperation1 = &aws.Operation{
			Name: "OperationName",
		}
	}

	req = aws.NewRequest(c.Service, opInputService1TestCaseOperation1, input, output)
	output = &InputService1TestShapeInputService1TestCaseOperation1Output{}
	req.Data = output
	return
}

func (c *InputService1ProtocolTest) InputService1TestCaseOperation1(input *InputService1TestShapeInputShape) (output *InputService1TestShapeInputService1TestCaseOperation1Output, err error) {
	req, out := c.InputService1TestCaseOperation1Request(input)
	output = out
	err = req.Send()
	return
}

var opInputService1TestCaseOperation1 *aws.Operation

type InputService1TestShapeInputService1TestCaseOperation1Output struct {
	metadataInputService1TestShapeInputService1TestCaseOperation1Output `json:"-", xml:"-"`
}

type metadataInputService1TestShapeInputService1TestCaseOperation1Output struct {
	SDKShapeTraits bool `type:"structure"`
}

type InputService1TestShapeInputShape struct {
	Bar *string `type:"string"`

	Foo *string `type:"string"`

	metadataInputService1TestShapeInputShape `json:"-", xml:"-"`
}

type metadataInputService1TestShapeInputShape struct {
	SDKShapeTraits bool `type:"structure"`
}

// InputService2ProtocolTest is a client for InputService2ProtocolTest.
type InputService2ProtocolTest struct {
	*aws.Service
}

// New returns a new InputService2ProtocolTest client.
func NewInputService2ProtocolTest(config *aws.Config) *InputService2ProtocolTest {
	if config == nil {
		config = &aws.Config{}
	}

	service := &aws.Service{
		Config:      aws.DefaultConfig.Merge(config),
		ServiceName: "inputservice2protocoltest",
		APIVersion:  "2014-01-01",
	}
	service.Initialize()

	// Handlers
	service.Handlers.Sign.PushBack(v4.Sign)
	service.Handlers.Build.PushBack(ec2query.Build)
	service.Handlers.Unmarshal.PushBack(ec2query.Unmarshal)
	service.Handlers.UnmarshalMeta.PushBack(ec2query.UnmarshalMeta)
	service.Handlers.UnmarshalError.PushBack(ec2query.UnmarshalError)

	return &InputService2ProtocolTest{service}
}

// InputService2TestCaseOperation1Request generates a request for the InputService2TestCaseOperation1 operation.
func (c *InputService2ProtocolTest) InputService2TestCaseOperation1Request(input *InputService2TestShapeInputShape) (req *aws.Request, output *InputService2TestShapeInputService2TestCaseOperation1Output) {
	if opInputService2TestCaseOperation1 == nil {
		opInputService2TestCaseOperation1 = &aws.Operation{
			Name: "OperationName",
		}
	}

	req = aws.NewRequest(c.Service, opInputService2TestCaseOperation1, input, output)
	output = &InputService2TestShapeInputService2TestCaseOperation1Output{}
	req.Data = output
	return
}

func (c *InputService2ProtocolTest) InputService2TestCaseOperation1(input *InputService2TestShapeInputShape) (output *InputService2TestShapeInputService2TestCaseOperation1Output, err error) {
	req, out := c.InputService2TestCaseOperation1Request(input)
	output = out
	err = req.Send()
	return
}

var opInputService2TestCaseOperation1 *aws.Operation

type InputService2TestShapeInputService2TestCaseOperation1Output struct {
	metadataInputService2TestShapeInputService2TestCaseOperation1Output `json:"-", xml:"-"`
}

type metadataInputService2TestShapeInputService2TestCaseOperation1Output struct {
	SDKShapeTraits bool `type:"structure"`
}

type InputService2TestShapeInputShape struct {
	Bar *string `locationName:"barLocationName" type:"string"`

	Foo *string `type:"string"`

	Yuck *string `locationName:"yuckLocationName" queryName:"yuckQueryName" type:"string"`

	metadataInputService2TestShapeInputShape `json:"-", xml:"-"`
}

type metadataInputService2TestShapeInputShape struct {
	SDKShapeTraits bool `type:"structure"`
}

// InputService3ProtocolTest is a client for InputService3ProtocolTest.
type InputService3ProtocolTest struct {
	*aws.Service
}

// New returns a new InputService3ProtocolTest client.
func NewInputService3ProtocolTest(config *aws.Config) *InputService3ProtocolTest {
	if config == nil {
		config = &aws.Config{}
	}

	service := &aws.Service{
		Config:      aws.DefaultConfig.Merge(config),
		ServiceName: "inputservice3protocoltest",
		APIVersion:  "2014-01-01",
	}
	service.Initialize()

	// Handlers
	service.Handlers.Sign.PushBack(v4.Sign)
	service.Handlers.Build.PushBack(ec2query.Build)
	service.Handlers.Unmarshal.PushBack(ec2query.Unmarshal)
	service.Handlers.UnmarshalMeta.PushBack(ec2query.UnmarshalMeta)
	service.Handlers.UnmarshalError.PushBack(ec2query.UnmarshalError)

	return &InputService3ProtocolTest{service}
}

// InputService3TestCaseOperation1Request generates a request for the InputService3TestCaseOperation1 operation.
func (c *InputService3ProtocolTest) InputService3TestCaseOperation1Request(input *InputService3TestShapeInputShape) (req *aws.Request, output *InputService3TestShapeInputService3TestCaseOperation1Output) {
	if opInputService3TestCaseOperation1 == nil {
		opInputService3TestCaseOperation1 = &aws.Operation{
			Name: "OperationName",
		}
	}

	req = aws.NewRequest(c.Service, opInputService3TestCaseOperation1, input, output)
	output = &InputService3TestShapeInputService3TestCaseOperation1Output{}
	req.Data = output
	return
}

func (c *InputService3ProtocolTest) InputService3TestCaseOperation1(input *InputService3TestShapeInputShape) (output *InputService3TestShapeInputService3TestCaseOperation1Output, err error) {
	req, out := c.InputService3TestCaseOperation1Request(input)
	output = out
	err = req.Send()
	return
}

var opInputService3TestCaseOperation1 *aws.Operation

type InputService3TestShapeInputService3TestCaseOperation1Output struct {
	metadataInputService3TestShapeInputService3TestCaseOperation1Output `json:"-", xml:"-"`
}

type metadataInputService3TestShapeInputService3TestCaseOperation1Output struct {
	SDKShapeTraits bool `type:"structure"`
}

type InputService3TestShapeInputShape struct {
	StructArg *InputService3TestShapeStructType `locationName:"Struct" type:"structure"`

	metadataInputService3TestShapeInputShape `json:"-", xml:"-"`
}

type metadataInputService3TestShapeInputShape struct {
	SDKShapeTraits bool `type:"structure"`
}

type InputService3TestShapeStructType struct {
	ScalarArg *string `locationName:"Scalar" type:"string"`

	metadataInputService3TestShapeStructType `json:"-", xml:"-"`
}

type metadataInputService3TestShapeStructType struct {
	SDKShapeTraits bool `type:"structure"`
}

// InputService4ProtocolTest is a client for InputService4ProtocolTest.
type InputService4ProtocolTest struct {
	*aws.Service
}

// New returns a new InputService4ProtocolTest client.
func NewInputService4ProtocolTest(config *aws.Config) *InputService4ProtocolTest {
	if config == nil {
		config = &aws.Config{}
	}

	service := &aws.Service{
		Config:      aws.DefaultConfig.Merge(config),
		ServiceName: "inputservice4protocoltest",
		APIVersion:  "2014-01-01",
	}
	service.Initialize()

	// Handlers
	service.Handlers.Sign.PushBack(v4.Sign)
	service.Handlers.Build.PushBack(ec2query.Build)
	service.Handlers.Unmarshal.PushBack(ec2query.Unmarshal)
	service.Handlers.UnmarshalMeta.PushBack(ec2query.UnmarshalMeta)
	service.Handlers.UnmarshalError.PushBack(ec2query.UnmarshalError)

	return &InputService4ProtocolTest{service}
}

// InputService4TestCaseOperation1Request generates a request for the InputService4TestCaseOperation1 operation.
func (c *InputService4ProtocolTest) InputService4TestCaseOperation1Request(input *InputService4TestShapeInputShape) (req *aws.Request, output *InputService4TestShapeInputService4TestCaseOperation1Output) {
	if opInputService4TestCaseOperation1 == nil {
		opInputService4TestCaseOperation1 = &aws.Operation{
			Name: "OperationName",
		}
	}

	req = aws.NewRequest(c.Service, opInputService4TestCaseOperation1, input, output)
	output = &InputService4TestShapeInputService4TestCaseOperation1Output{}
	req.Data = output
	return
}

func (c *InputService4ProtocolTest) InputService4TestCaseOperation1(input *InputService4TestShapeInputShape) (output *InputService4TestShapeInputService4TestCaseOperation1Output, err error) {
	req, out := c.InputService4TestCaseOperation1Request(input)
	output = out
	err = req.Send()
	return
}

var opInputService4TestCaseOperation1 *aws.Operation

type InputService4TestShapeInputService4TestCaseOperation1Output struct {
	metadataInputService4TestShapeInputService4TestCaseOperation1Output `json:"-", xml:"-"`
}

type metadataInputService4TestShapeInputService4TestCaseOperation1Output struct {
	SDKShapeTraits bool `type:"structure"`
}

type InputService4TestShapeInputShape struct {
	ListArg []*string `type:"list"`

	metadataInputService4TestShapeInputShape `json:"-", xml:"-"`
}

type metadataInputService4TestShapeInputShape struct {
	SDKShapeTraits bool `type:"structure"`
}

// InputService5ProtocolTest is a client for InputService5ProtocolTest.
type InputService5ProtocolTest struct {
	*aws.Service
}

// New returns a new InputService5ProtocolTest client.
func NewInputService5ProtocolTest(config *aws.Config) *InputService5ProtocolTest {
	if config == nil {
		config = &aws.Config{}
	}

	service := &aws.Service{
		Config:      aws.DefaultConfig.Merge(config),
		ServiceName: "inputservice5protocoltest",
		APIVersion:  "2014-01-01",
	}
	service.Initialize()

	// Handlers
	service.Handlers.Sign.PushBack(v4.Sign)
	service.Handlers.Build.PushBack(ec2query.Build)
	service.Handlers.Unmarshal.PushBack(ec2query.Unmarshal)
	service.Handlers.UnmarshalMeta.PushBack(ec2query.UnmarshalMeta)
	service.Handlers.UnmarshalError.PushBack(ec2query.UnmarshalError)

	return &InputService5ProtocolTest{service}
}

// InputService5TestCaseOperation1Request generates a request for the InputService5TestCaseOperation1 operation.
func (c *InputService5ProtocolTest) InputService5TestCaseOperation1Request(input *InputService5TestShapeInputShape) (req *aws.Request, output *InputService5TestShapeInputService5TestCaseOperation1Output) {
	if opInputService5TestCaseOperation1 == nil {
		opInputService5TestCaseOperation1 = &aws.Operation{
			Name: "OperationName",
		}
	}

	req = aws.NewRequest(c.Service, opInputService5TestCaseOperation1, input, output)
	output = &InputService5TestShapeInputService5TestCaseOperation1Output{}
	req.Data = output
	return
}

func (c *InputService5ProtocolTest) InputService5TestCaseOperation1(input *InputService5TestShapeInputShape) (output *InputService5TestShapeInputService5TestCaseOperation1Output, err error) {
	req, out := c.InputService5TestCaseOperation1Request(input)
	output = out
	err = req.Send()
	return
}

var opInputService5TestCaseOperation1 *aws.Operation

type InputService5TestShapeInputService5TestCaseOperation1Output struct {
	metadataInputService5TestShapeInputService5TestCaseOperation1Output `json:"-", xml:"-"`
}

type metadataInputService5TestShapeInputService5TestCaseOperation1Output struct {
	SDKShapeTraits bool `type:"structure"`
}

type InputService5TestShapeInputShape struct {
	ListArg []*string `locationName:"ListMemberName" locationNameList:"item" type:"list"`

	metadataInputService5TestShapeInputShape `json:"-", xml:"-"`
}

type metadataInputService5TestShapeInputShape struct {
	SDKShapeTraits bool `type:"structure"`
}

// InputService6ProtocolTest is a client for InputService6ProtocolTest.
type InputService6ProtocolTest struct {
	*aws.Service
}

// New returns a new InputService6ProtocolTest client.
func NewInputService6ProtocolTest(config *aws.Config) *InputService6ProtocolTest {
	if config == nil {
		config = &aws.Config{}
	}

	service := &aws.Service{
		Config:      aws.DefaultConfig.Merge(config),
		ServiceName: "inputservice6protocoltest",
		APIVersion:  "2014-01-01",
	}
	service.Initialize()

	// Handlers
	service.Handlers.Sign.PushBack(v4.Sign)
	service.Handlers.Build.PushBack(ec2query.Build)
	service.Handlers.Unmarshal.PushBack(ec2query.Unmarshal)
	service.Handlers.UnmarshalMeta.PushBack(ec2query.UnmarshalMeta)
	service.Handlers.UnmarshalError.PushBack(ec2query.UnmarshalError)

	return &InputService6ProtocolTest{service}
}

// InputService6TestCaseOperation1Request generates a request for the InputService6TestCaseOperation1 operation.
func (c *InputService6ProtocolTest) InputService6TestCaseOperation1Request(input *InputService6TestShapeInputShape) (req *aws.Request, output *InputService6TestShapeInputService6TestCaseOperation1Output) {
	if opInputService6TestCaseOperation1 == nil {
		opInputService6TestCaseOperation1 = &aws.Operation{
			Name: "OperationName",
		}
	}

	req = aws.NewRequest(c.Service, opInputService6TestCaseOperation1, input, output)
	output = &InputService6TestShapeInputService6TestCaseOperation1Output{}
	req.Data = output
	return
}

func (c *InputService6ProtocolTest) InputService6TestCaseOperation1(input *InputService6TestShapeInputShape) (output *InputService6TestShapeInputService6TestCaseOperation1Output, err error) {
	req, out := c.InputService6TestCaseOperation1Request(input)
	output = out
	err = req.Send()
	return
}

var opInputService6TestCaseOperation1 *aws.Operation

type InputService6TestShapeInputService6TestCaseOperation1Output struct {
	metadataInputService6TestShapeInputService6TestCaseOperation1Output `json:"-", xml:"-"`
}

type metadataInputService6TestShapeInputService6TestCaseOperation1Output struct {
	SDKShapeTraits bool `type:"structure"`
}

type InputService6TestShapeInputShape struct {
	ListArg []*string `locationName:"ListMemberName" queryName:"ListQueryName" locationNameList:"item" type:"list"`

	metadataInputService6TestShapeInputShape `json:"-", xml:"-"`
}

type metadataInputService6TestShapeInputShape struct {
	SDKShapeTraits bool `type:"structure"`
}

// InputService7ProtocolTest is a client for InputService7ProtocolTest.
type InputService7ProtocolTest struct {
	*aws.Service
}

// New returns a new InputService7ProtocolTest client.
func NewInputService7ProtocolTest(config *aws.Config) *InputService7ProtocolTest {
	if config == nil {
		config = &aws.Config{}
	}

	service := &aws.Service{
		Config:      aws.DefaultConfig.Merge(config),
		ServiceName: "inputservice7protocoltest",
		APIVersion:  "2014-01-01",
	}
	service.Initialize()

	// Handlers
	service.Handlers.Sign.PushBack(v4.Sign)
	service.Handlers.Build.PushBack(ec2query.Build)
	service.Handlers.Unmarshal.PushBack(ec2query.Unmarshal)
	service.Handlers.UnmarshalMeta.PushBack(ec2query.UnmarshalMeta)
	service.Handlers.UnmarshalError.PushBack(ec2query.UnmarshalError)

	return &InputService7ProtocolTest{service}
}

// InputService7TestCaseOperation1Request generates a request for the InputService7TestCaseOperation1 operation.
func (c *InputService7ProtocolTest) InputService7TestCaseOperation1Request(input *InputService7TestShapeInputShape) (req *aws.Request, output *InputService7TestShapeInputService7TestCaseOperation1Output) {
	if opInputService7TestCaseOperation1 == nil {
		opInputService7TestCaseOperation1 = &aws.Operation{
			Name: "OperationName",
		}
	}

	req = aws.NewRequest(c.Service, opInputService7TestCaseOperation1, input, output)
	output = &InputService7TestShapeInputService7TestCaseOperation1Output{}
	req.Data = output
	return
}

func (c *InputService7ProtocolTest) InputService7TestCaseOperation1(input *InputService7TestShapeInputShape) (output *InputService7TestShapeInputService7TestCaseOperation1Output, err error) {
	req, out := c.InputService7TestCaseOperation1Request(input)
	output = out
	err = req.Send()
	return
}

var opInputService7TestCaseOperation1 *aws.Operation

type InputService7TestShapeInputService7TestCaseOperation1Output struct {
	metadataInputService7TestShapeInputService7TestCaseOperation1Output `json:"-", xml:"-"`
}

type metadataInputService7TestShapeInputService7TestCaseOperation1Output struct {
	SDKShapeTraits bool `type:"structure"`
}

type InputService7TestShapeInputShape struct {
	BlobArg []byte `type:"blob"`

	metadataInputService7TestShapeInputShape `json:"-", xml:"-"`
}

type metadataInputService7TestShapeInputShape struct {
	SDKShapeTraits bool `type:"structure"`
}

// InputService8ProtocolTest is a client for InputService8ProtocolTest.
type InputService8ProtocolTest struct {
	*aws.Service
}

// New returns a new InputService8ProtocolTest client.
func NewInputService8ProtocolTest(config *aws.Config) *InputService8ProtocolTest {
	if config == nil {
		config = &aws.Config{}
	}

	service := &aws.Service{
		Config:      aws.DefaultConfig.Merge(config),
		ServiceName: "inputservice8protocoltest",
		APIVersion:  "2014-01-01",
	}
	service.Initialize()

	// Handlers
	service.Handlers.Sign.PushBack(v4.Sign)
	service.Handlers.Build.PushBack(ec2query.Build)
	service.Handlers.Unmarshal.PushBack(ec2query.Unmarshal)
	service.Handlers.UnmarshalMeta.PushBack(ec2query.UnmarshalMeta)
	service.Handlers.UnmarshalError.PushBack(ec2query.UnmarshalError)

	return &InputService8ProtocolTest{service}
}

// InputService8TestCaseOperation1Request generates a request for the InputService8TestCaseOperation1 operation.
func (c *InputService8ProtocolTest) InputService8TestCaseOperation1Request(input *InputService8TestShapeInputShape) (req *aws.Request, output *InputService8TestShapeInputService8TestCaseOperation1Output) {
	if opInputService8TestCaseOperation1 == nil {
		opInputService8TestCaseOperation1 = &aws.Operation{
			Name: "OperationName",
		}
	}

	req = aws.NewRequest(c.Service, opInputService8TestCaseOperation1, input, output)
	output = &InputService8TestShapeInputService8TestCaseOperation1Output{}
	req.Data = output
	return
}

func (c *InputService8ProtocolTest) InputService8TestCaseOperation1(input *InputService8TestShapeInputShape) (output *InputService8TestShapeInputService8TestCaseOperation1Output, err error) {
	req, out := c.InputService8TestCaseOperation1Request(input)
	output = out
	err = req.Send()
	return
}

var opInputService8TestCaseOperation1 *aws.Operation

type InputService8TestShapeInputService8TestCaseOperation1Output struct {
	metadataInputService8TestShapeInputService8TestCaseOperation1Output `json:"-", xml:"-"`
}

type metadataInputService8TestShapeInputService8TestCaseOperation1Output struct {
	SDKShapeTraits bool `type:"structure"`
}

type InputService8TestShapeInputShape struct {
	TimeArg *time.Time `type:"timestamp" timestampFormat:"iso8601"`

	metadataInputService8TestShapeInputShape `json:"-", xml:"-"`
}

type metadataInputService8TestShapeInputShape struct {
	SDKShapeTraits bool `type:"structure"`
}

//
// Tests begin here
//

func TestInputService1ProtocolTestScalarMembersCase1(t *testing.T) {
	svc := NewInputService1ProtocolTest(nil)
	svc.Endpoint = "https://test"

	input := &InputService1TestShapeInputShape{
		Bar: aws.String("val2"),
		Foo: aws.String("val1"),
	}
	req, _ := svc.InputService1TestCaseOperation1Request(input)
	r := req.HTTPRequest

	// build request
	ec2query.Build(req)
	assert.NoError(t, req.Error)

	// assert body
	assert.NotNil(t, r.Body)
	body, _ := ioutil.ReadAll(r.Body)
	assert.Equal(t, util.Trim(`Action=OperationName&Bar=val2&Foo=val1&Version=2014-01-01`), util.Trim(string(body)))

	// assert URL
	assert.Equal(t, "https://test/", r.URL.String())

	// assert headers

}

func TestInputService2ProtocolTestStructureWithLocationNameAndQueryNameAppliedToMembersCase1(t *testing.T) {
	svc := NewInputService2ProtocolTest(nil)
	svc.Endpoint = "https://test"

	input := &InputService2TestShapeInputShape{
		Bar:  aws.String("val2"),
		Foo:  aws.String("val1"),
		Yuck: aws.String("val3"),
	}
	req, _ := svc.InputService2TestCaseOperation1Request(input)
	r := req.HTTPRequest

	// build request
	ec2query.Build(req)
	assert.NoError(t, req.Error)

	// assert body
	assert.NotNil(t, r.Body)
	body, _ := ioutil.ReadAll(r.Body)
	assert.Equal(t, util.Trim(`Action=OperationName&BarLocationName=val2&Foo=val1&Version=2014-01-01&yuckQueryName=val3`), util.Trim(string(body)))

	// assert URL
	assert.Equal(t, "https://test/", r.URL.String())

	// assert headers

}

func TestInputService3ProtocolTestNestedStructureMembersCase1(t *testing.T) {
	svc := NewInputService3ProtocolTest(nil)
	svc.Endpoint = "https://test"

	input := &InputService3TestShapeInputShape{
		StructArg: &InputService3TestShapeStructType{
			ScalarArg: aws.String("foo"),
		},
	}
	req, _ := svc.InputService3TestCaseOperation1Request(input)
	r := req.HTTPRequest

	// build request
	ec2query.Build(req)
	assert.NoError(t, req.Error)

	// assert body
	assert.NotNil(t, r.Body)
	body, _ := ioutil.ReadAll(r.Body)
	assert.Equal(t, util.Trim(`Action=OperationName&Struct.Scalar=foo&Version=2014-01-01`), util.Trim(string(body)))

	// assert URL
	assert.Equal(t, "https://test/", r.URL.String())

	// assert headers

}

func TestInputService4ProtocolTestListTypesCase1(t *testing.T) {
	svc := NewInputService4ProtocolTest(nil)
	svc.Endpoint = "https://test"

	input := &InputService4TestShapeInputShape{
		ListArg: []*string{
			aws.String("foo"),
			aws.String("bar"),
			aws.String("baz"),
		},
	}
	req, _ := svc.InputService4TestCaseOperation1Request(input)
	r := req.HTTPRequest

	// build request
	ec2query.Build(req)
	assert.NoError(t, req.Error)

	// assert body
	assert.NotNil(t, r.Body)
	body, _ := ioutil.ReadAll(r.Body)
	assert.Equal(t, util.Trim(`Action=OperationName&ListArg.1=foo&ListArg.2=bar&ListArg.3=baz&Version=2014-01-01`), util.Trim(string(body)))

	// assert URL
	assert.Equal(t, "https://test/", r.URL.String())

	// assert headers

}

func TestInputService5ProtocolTestListWithLocationNameAppliedToMemberCase1(t *testing.T) {
	svc := NewInputService5ProtocolTest(nil)
	svc.Endpoint = "https://test"

	input := &InputService5TestShapeInputShape{
		ListArg: []*string{
			aws.String("a"),
			aws.String("b"),
			aws.String("c"),
		},
	}
	req, _ := svc.InputService5TestCaseOperation1Request(input)
	r := req.HTTPRequest

	// build request
	ec2query.Build(req)
	assert.NoError(t, req.Error)

	// assert body
	assert.NotNil(t, r.Body)
	body, _ := ioutil.ReadAll(r.Body)
	assert.Equal(t, util.Trim(`Action=OperationName&ListMemberName.1=a&ListMemberName.2=b&ListMemberName.3=c&Version=2014-01-01`), util.Trim(string(body)))

	// assert URL
	assert.Equal(t, "https://test/", r.URL.String())

	// assert headers

}

func TestInputService6ProtocolTestListWithLocationNameAndQueryNameCase1(t *testing.T) {
	svc := NewInputService6ProtocolTest(nil)
	svc.Endpoint = "https://test"

	input := &InputService6TestShapeInputShape{
		ListArg: []*string{
			aws.String("a"),
			aws.String("b"),
			aws.String("c"),
		},
	}
	req, _ := svc.InputService6TestCaseOperation1Request(input)
	r := req.HTTPRequest

	// build request
	ec2query.Build(req)
	assert.NoError(t, req.Error)

	// assert body
	assert.NotNil(t, r.Body)
	body, _ := ioutil.ReadAll(r.Body)
	assert.Equal(t, util.Trim(`Action=OperationName&ListQueryName.1=a&ListQueryName.2=b&ListQueryName.3=c&Version=2014-01-01`), util.Trim(string(body)))

	// assert URL
	assert.Equal(t, "https://test/", r.URL.String())

	// assert headers

}

func TestInputService7ProtocolTestBase64EncodedBlobsCase1(t *testing.T) {
	svc := NewInputService7ProtocolTest(nil)
	svc.Endpoint = "https://test"

	input := &InputService7TestShapeInputShape{
		BlobArg: []byte("foo"),
	}
	req, _ := svc.InputService7TestCaseOperation1Request(input)
	r := req.HTTPRequest

	// build request
	ec2query.Build(req)
	assert.NoError(t, req.Error)

	// assert body
	assert.NotNil(t, r.Body)
	body, _ := ioutil.ReadAll(r.Body)
	assert.Equal(t, util.Trim(`Action=OperationName&BlobArg=Zm9v&Version=2014-01-01`), util.Trim(string(body)))

	// assert URL
	assert.Equal(t, "https://test/", r.URL.String())

	// assert headers

}

func TestInputService8ProtocolTestTimestampValuesCase1(t *testing.T) {
	svc := NewInputService8ProtocolTest(nil)
	svc.Endpoint = "https://test"

	input := &InputService8TestShapeInputShape{
		TimeArg: aws.Time(time.Unix(1422172800, 0)),
	}
	req, _ := svc.InputService8TestCaseOperation1Request(input)
	r := req.HTTPRequest

	// build request
	ec2query.Build(req)
	assert.NoError(t, req.Error)

	// assert body
	assert.NotNil(t, r.Body)
	body, _ := ioutil.ReadAll(r.Body)
	assert.Equal(t, util.Trim(`Action=OperationName&TimeArg=2015-01-25T08%3A00%3A00Z&Version=2014-01-01`), util.Trim(string(body)))

	// assert URL
	assert.Equal(t, "https://test/", r.URL.String())

	// assert headers

}
