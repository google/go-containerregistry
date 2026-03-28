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

// canonicalAddr returns url.Host but always lower-cased with a ":port" suffix
// FROM: http://golang.org/src/net/http/transport.go
func canonicalAddr(url *url.URL) string {
	addr := strings.ToLower(url.Host)
	if !hasPort(addr) {
		return addr + ":" + portMap[url.Scheme]
	}
	return addr
}

// normalizedURL returns the endpoint URL in canonical form.
func normalizedURL(endpoint url.URL) string {
	endpoint.Host = canonicalAddr(&endpoint)
	return endpoint.String()
}

type simpleManager struct {
	mu         sync.RWMutex
	challenges map[string][]Challenge
}

func (m *simpleManager) GetChallenges(endpoint url.URL) ([]Challenge, error) {
	key := normalizedURL(endpoint)

	m.mu.RLock()
	challenges := m.challenges[key]
	m.mu.RUnlock()

	return challenges, nil
}

func (m *simpleManager) AddResponse(resp *http.Response) error {
	if resp.Request == nil {
		return fmt.Errorf("missing request reference")
	}
	key := normalizedURL(url.URL{
		Path:   resp.Request.URL.Path,
		Host:   resp.Request.URL.Host,
		Scheme: resp.Request.URL.Scheme,
	})

	challenges := ResponseChallenges(resp)
	m.mu.Lock()
	if m.challenges == nil {
		m.challenges = make(map[string][]Challenge)
	}
	m.challenges[key] = challenges
	m.mu.Unlock()

	return nil
}
