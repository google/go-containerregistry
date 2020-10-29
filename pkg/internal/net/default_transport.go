package net

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// NewDefaultTransport is a an extension of the default net/http DefaultTransport with http2 disabled.
// According to moby/buildkit#1420, net/http's lack of tunable flow control prevents full throughput on push.
func NewDefaultTransport() http.RoundTripper {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSNextProto:          make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}
}
