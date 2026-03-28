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

// Manager manages the challenges for endpoints.
// The challenges are pulled out of HTTP responses. Only
// responses which expect challenges should be added to
// the manager, since a non-unauthorized request will be
// viewed as not requiring challenges.
type Manager interface {
	// GetChallenges returns the challenges for the given
	// endpoint URL.
	GetChallenges(endpoint url.URL) ([]Challenge, error)

	// AddResponse adds the response to the challenge
	// manager. The challenges will be parsed out of
	// the WWW-Authenticate headers and added to the
	// URL which was produced the response. If the
	// response was authorized, any challenges for the
	// endpoint will be cleared.
	AddResponse(resp *http.Response) error
}

// NewSimpleManager returns an instance of
// Manager which only maps endpoints to challenges
// based on the responses which have been added the
// manager. The simple manager will make no attempt to
// perform requests on the endpoints or cache the responses
// to a backend.
func NewSimpleManager() Manager {
	return &simpleManager{
		Challenges: make(map[string][]Challenge),
	}
}

type simpleManager struct {
	sync.RWMutex
	Challenges map[string][]Challenge
}

func normalizeURL(endpoint *url.URL) {
	endpoint.Host = strings.ToLower(endpoint.Host)
	endpoint.Host = canonicalAddr(endpoint)
}

func (m *simpleManager) GetChallenges(endpoint url.URL) ([]Challenge, error) {
	normalizeURL(&endpoint)

	m.RLock()
	defer m.RUnlock()
	challenges := m.Challenges[endpoint.String()]
	return challenges, nil
}

func (m *simpleManager) AddResponse(resp *http.Response) error {
	challenges := ResponseChallenges(resp)
	if resp.Request == nil {
		return fmt.Errorf("missing request reference")
	}
	urlCopy := url.URL{
		Path:   resp.Request.URL.Path,
		Host:   resp.Request.URL.Host,
		Scheme: resp.Request.URL.Scheme,
	}
	normalizeURL(&urlCopy)

	m.Lock()
	defer m.Unlock()
	m.Challenges[urlCopy.String()] = challenges
	return nil
}
