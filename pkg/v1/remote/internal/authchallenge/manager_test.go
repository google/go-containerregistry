package authchallenge

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// FROM: https://golang.org/src/net/http/http.go
// Given a string of the form "host", "host:port", or "[ipv6::address]:port",
// return true if the string includes a port.
func hasPort(s string) bool { return strings.LastIndex(s, ":") > strings.LastIndex(s, "]") }

// FROM: http://golang.org/src/net/http/transport.go
var portMap = map[string]string{
	"http":  "80",
	"https": "443",
}

// canonicalAddr returns url.Host but always with a ":port" suffix
// FROM: http://golang.org/src/net/http/transport.go
func canonicalAddr(url *url.URL) string {
	addr := url.Host
	if !hasPort(addr) {
		return addr + ":" + portMap[url.Scheme]
	}
	return addr
}

type simpleManager struct {
	mu         sync.RWMutex
	challenges map[string][]Challenge
}

func normalizeURL(endpoint *url.URL) {
	endpoint.Host = strings.ToLower(endpoint.Host)
	endpoint.Host = canonicalAddr(endpoint)
}

func (m *simpleManager) GetChallenges(endpoint url.URL) ([]Challenge, error) {
	normalizeURL(&endpoint)

	m.mu.RLock()
	challenges := m.challenges[endpoint.String()]
	m.mu.RUnlock()

	return challenges, nil
}

func (m *simpleManager) AddResponse(resp *http.Response) error {
	if resp.Request == nil {
		return fmt.Errorf("missing request reference")
	}
	urlCopy := url.URL{
		Path:   resp.Request.URL.Path,
		Host:   resp.Request.URL.Host,
		Scheme: resp.Request.URL.Scheme,
	}
	normalizeURL(&urlCopy)

	challenges := ResponseChallenges(resp)
	m.mu.Lock()
	if m.challenges == nil {
		m.challenges = make(map[string][]Challenge)
	}
	m.challenges[urlCopy.String()] = challenges
	m.mu.Unlock()

	return nil
}
