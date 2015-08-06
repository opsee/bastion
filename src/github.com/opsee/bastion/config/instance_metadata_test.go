package config

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"net/http"
	"testing"
)

const instanceResponseTxt = `{
  "instanceId" : "i-e58b75eb",
  "billingProducts" : null,
  "architecture" : "x86_64",
  "imageId" : "ami-b5a7ea85",
  "pendingTime" : "2014-12-17T00:52:02Z",
  "instanceType" : "t2.micro",
  "accountId" : "933693344490",
  "kernelId" : null,
  "ramdiskId" : null,
  "region" : "us-west-2",
  "version" : "2010-08-31",
  "privateIp" : "172.31.8.109",
  "availabilityZone" : "us-west-2c",
  "devpayProductCodes" : null
}`

type MockedHttpClient struct {
	mock.Mock
}

type MockedReaderCloser struct {
	body   string
	idx    int
	closed bool
}

func (m *MockedHttpClient) Get(url string) (resp *http.Response, err error) {
	args := m.Mock.Called(url)
	return args.Get(0).(*http.Response), args.Error(1)
}

func (m *MockedReaderCloser) Read(p []byte) (n int, err error) {
	ba := []byte(m.body)
	if len(p)+m.idx >= len(m.body) {
		dst := p[0 : len(p)-m.idx]
		src := ba[m.idx:len(ba)]
		n := copy(dst, src)
		m.idx += n
		return n, io.EOF
	} else {
		src := ba[m.idx:len(ba)]
		n = copy(p, src)
		m.idx += n
		return n, nil
	}
}

func (m *MockedReaderCloser) Close() error {
	m.closed = true
	return nil
}

func setupMock(m *MockedHttpClient) {
	m.On("Get", MetadataURL).Return(&http.Response{
		Body:       &MockedReaderCloser{body: instanceResponseTxt},
		StatusCode: 200}, nil)
}

func TestGetInstanceId(t *testing.T) {
	m := &MockedHttpClient{}
	setupMock(m)
	metap := &MetadataProvider{client: m}
	metadata := metap.Get()
	m.Mock.AssertExpectations(t)
	assert.Equal(t, "i-e58b75eb", metadata.InstanceId)
}
