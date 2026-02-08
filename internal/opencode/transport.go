package opencode

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// NewProxyTransport creates an HTTP transport with proxy support.
// Supports socks5://host:port, http://host:port, https://host:port
// Returns default transport if proxyURL is empty.
func NewProxyTransport(proxyURL string) (*http.Transport, error) {
	if proxyURL == "" {
		return &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		}, nil
	}

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("parse proxy URL: %w", err)
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	switch parsedURL.Scheme {
	case "socks5":
		// Use golang.org/x/net/proxy for SOCKS5
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, nil, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("create SOCKS5 proxy: %w", err)
		}
		transport.Dial = dialer.Dial
		// For CONNECT tunneling (HTTPS over SOCKS5)
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}

	case "http", "https":
		// Use standard HTTP proxy
		transport.Proxy = http.ProxyURL(parsedURL)

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", parsedURL.Scheme)
	}

	return transport, nil
}

// NewHTTPClient creates an HTTP client with optional proxy support
func NewHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	transport, err := NewProxyTransport(proxyURL)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}
