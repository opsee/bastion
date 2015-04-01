package credentials

import (
	"io"
	"time"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/stretchr/testify/assert"
	"github.com/opsee/bastion/Godeps/_workspace/src/github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/opsee/bastion/testing"
	"net/http"
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

const credentialsResponseTxt = `{
  "Code" : "Success",
  "LastUpdated" : "2014-12-29T00:43:07Z",
  "Type" : "AWS-HMAC",
  "AccessKeyId" : "blahblah",
  "SecretAccessKey" : "masdfasdfasdfasdfasdf",
  "Token" : "AQoDYXdzECIa4AMkW+AKdJIZZ+aasdfasdfasdf++DHh7Z0hhm+asdfasdfasdf+BylV4YCKpRhUKlblQp16eyhY7iR1SXyyNK2JZoDSm00yDgJ7FxBDBc0OalKF4o+JUKbwKVRwooUUl7VNyOVcQctLL0eKwIWxjE+BRe6SNhxYU7koNC+C1DfXzjYyD6cAgz6dvrZ7/oFf8FEW5jorSRJgTdDGygZA81pT670b++aYYsxjwGQLX9tsQ1txZOmfF/BzEmM1a6v8Me5FFuGizRythUJ88Hw+3MDlcEgNgoldpwJXCa61Ly2RYZdOEIB9dFNHRZqjQNnTwzscB2mO2bh47BlIN55PUVi5izrwC79kCjj4Wp98qio4yIlOmMn8HJEYmjCl5QB/PFpfdRDr9uEkttyVFgWPbQz/eyHiNrEPfVrJ3zcQcoDz8Ryxpz1So7MR7qLqaCL38CzTvdMB+5TASZTz4nVpeplppJgmDJkSV82XSoQ54XUlMOuqaNfyXod9knrbJEyP7Q704xw0FTjIq2vh+wskG8c+qIgzMeCpQU=",
  "Expiration" : "2014-12-29T07:06:55Z"
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
	m.On("Get", "http://169.254.169.254/latest/dynamic/instance-identity/document").Return(&http.Response{
		Body:       &MockedReaderCloser{body: instanceResponseTxt},
		StatusCode: 200}, nil)
	m.On("Get", "http://169.254.169.254/latest/meta-data/iam/security-credentials/opsee").Return(&http.Response{
		Body:       &MockedReaderCloser{body: credentialsResponseTxt},
		StatusCode: 200}, nil)
}

func TestGetInstanceId(t *testing.T) {
	m := &MockedHttpClient{}
	setupMock(m)
	cp := NewProvider(m, "", "", "")
	time.Sleep(time.Millisecond * 100)
	m.Mock.AssertExpectations(t)

	id := cp.GetInstanceId()
	assert.Equal(t, "i-e58b75eb", id.InstanceId)
}

func TestGetCredentials(t *testing.T) {
	m := &MockedHttpClient{}
	setupMock(m)
	cp := NewProvider(m, "", "", "")
	time.Sleep(time.Millisecond * 100)
	m.Mock.AssertExpectations(t)
	creds := cp.Credentials()
	assert.Equal(t, "blahblah", creds.AccessKeyId)
	assert.Equal(t, "us-west-2", creds.Region)
	assert.Equal(t, "masdfasdfasdfasdfasdf", creds.SecretAccessKey)
}

func TestOverrideAccessKeyId(t *testing.T) {
	m := &MockedHttpClient{}
	setupMock(m)
	cp := NewProvider(m, "hello", "", "")
	time.Sleep(time.Millisecond * 100)
	m.Mock.AssertExpectations(t)

	creds := cp.Credentials()
	assert.Equal(t, "hello", creds.AccessKeyId)
	assert.Equal(t, "us-west-2", creds.Region)
	assert.Equal(t, "masdfasdfasdfasdfasdf", creds.SecretAccessKey)
}

func TestOverrideSecretAccessKey(t *testing.T) {
	m := &MockedHttpClient{}
	setupMock(m)
	cp := NewProvider(m, "", "hello", "")
	time.Sleep(time.Millisecond * 100)
	m.Mock.AssertExpectations(t)

	creds := cp.Credentials()
	assert.Equal(t, "blahblah", creds.AccessKeyId)
	assert.Equal(t, "us-west-2", creds.Region)
	assert.Equal(t, "hello", creds.SecretAccessKey)
}

func TestOverrideRegion(t *testing.T) {
	m := &MockedHttpClient{}
	setupMock(m)
	cp := NewProvider(m, "", "", "hello")
	time.Sleep(time.Millisecond * 100)
	m.Mock.AssertExpectations(t)

	creds := cp.Credentials()
	assert.Equal(t, "blahblah", creds.AccessKeyId)
	assert.Equal(t, "hello", creds.Region)
	assert.Equal(t, "masdfasdfasdfasdfasdf", creds.SecretAccessKey)
}

func TestOverrideAll(t *testing.T) {
	m := &MockedHttpClient{}
	// do not setup the mock since we expect no calls to be madee
	cp := NewProvider(m, "access", "secret", "region")
	time.Sleep(time.Millisecond * 100)

	creds := cp.Credentials()
	assert.Equal(t, "access", creds.AccessKeyId)
	assert.Equal(t, "secret", creds.SecretAccessKey)
	assert.Equal(t, "region", creds.Region)
}
