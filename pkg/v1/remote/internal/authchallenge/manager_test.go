// Copyright 2014 Docker, Inc.
// Copyright 2021-2026 The Distribution contributors
// Copyright 2026 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
