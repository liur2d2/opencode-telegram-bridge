package opencode

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProxyTransportEmpty(t *testing.T) {
	transport, err := NewProxyTransport("")
	require.NoError(t, err)
	require.NotNil(t, transport)
	assert.Nil(t, transport.Dial)
	assert.Nil(t, transport.Proxy)
}

func TestNewProxyTransportSOCKS5(t *testing.T) {
	transport, err := NewProxyTransport("socks5://localhost:1080")
	require.NoError(t, err)
	require.NotNil(t, transport)
	assert.NotNil(t, transport.Dial)
	assert.NotNil(t, transport.DialContext)
}

func TestNewProxyTransportHTTP(t *testing.T) {
	transport, err := NewProxyTransport("http://proxy.example.com:8080")
	require.NoError(t, err)
	require.NotNil(t, transport)
	assert.NotNil(t, transport.Proxy)
}

func TestNewProxyTransportHTTPS(t *testing.T) {
	transport, err := NewProxyTransport("https://proxy.example.com:8080")
	require.NoError(t, err)
	require.NotNil(t, transport)
	assert.NotNil(t, transport.Proxy)
}

func TestNewProxyTransportInvalidURL(t *testing.T) {
	_, err := NewProxyTransport("!invalid://url")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse proxy URL")
}

func TestNewProxyTransportUnsupportedScheme(t *testing.T) {
	_, err := NewProxyTransport("ftp://proxy.example.com:21")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported proxy scheme")
}

func TestNewHTTPClientWithoutProxy(t *testing.T) {
	client, err := NewHTTPClient("", 30*time.Second)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, 30*time.Second, client.Timeout)
	assert.NotNil(t, client.Transport)
}

func TestNewHTTPClientWithSOCKS5Proxy(t *testing.T) {
	client, err := NewHTTPClient("socks5://localhost:1080", 30*time.Second)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, 30*time.Second, client.Timeout)
	assert.NotNil(t, client.Transport)
	transport := client.Transport.(*http.Transport)
	assert.NotNil(t, transport.Dial)
}

func TestNewHTTPClientWithHTTPProxy(t *testing.T) {
	client, err := NewHTTPClient("http://proxy.example.com:8080", 60*time.Second)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, 60*time.Second, client.Timeout)
	assert.NotNil(t, client.Transport)
	transport := client.Transport.(*http.Transport)
	assert.NotNil(t, transport.Proxy)
}

func TestNewHTTPClientInvalidProxy(t *testing.T) {
	_, err := NewHTTPClient("!invalid://proxy", 30*time.Second)
	assert.Error(t, err)
}
