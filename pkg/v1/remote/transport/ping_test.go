// Copyright 2018 Google LLC All Rights Reserved.
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

package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
)

var (
	testRegistry, _ = name.NewRegistry("localhost:8080", name.StrictValidation)
)

func TestPingNoChallenge(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	pr, err := ping(context.Background(), testRegistry, tprt)
	if err != nil {
		t.Errorf("ping() = %v", err)
	}
	if pr.challenge != anonymous {
		t.Errorf("ping(); got %v, want %v", pr.challenge, anonymous)
	}
	if pr.scheme != "http" {
		t.Errorf("ping(); got %v, want %v", pr.scheme, "http")
	}
}

func TestPingBasicChallengeNoParams(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("WWW-Authenticate", `BASIC`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	pr, err := ping(context.Background(), testRegistry, tprt)
	if err != nil {
		t.Errorf("ping() = %v", err)
	}
	if pr.challenge != basic {
		t.Errorf("ping(); got %v, want %v", pr.challenge, basic)
	}
	if got, want := len(pr.parameters), 0; got != want {
		t.Errorf("ping(); got %v, want %v", got, want)
	}
}

func TestPingBearerChallengeWithParams(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="http://auth.example.com/token"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	pr, err := ping(context.Background(), testRegistry, tprt)
	if err != nil {
		t.Errorf("ping() = %v", err)
	}
	if pr.challenge != bearer {
		t.Errorf("ping(); got %v, want %v", pr.challenge, bearer)
	}
	if got, want := len(pr.parameters), 1; got != want {
		t.Errorf("ping(); got %v, want %v", got, want)
	}
}

func TestPingMultipleChallenges(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("WWW-Authenticate", "Negotiate")
			w.Header().Add("WWW-Authenticate", `Basic realm="http://auth.example.com/token"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	pr, err := ping(context.Background(), testRegistry, tprt)
	if err != nil {
		t.Errorf("ping() = %v", err)
	}
	if pr.challenge != basic {
		t.Errorf("ping(); got %v, want %v", pr.challenge, basic)
	}
	if got, want := len(pr.parameters), 1; got != want {
		t.Errorf("ping(); got %v, want %v", got, want)
	}
}

func TestPingMultipleNotSupportedChallenges(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("WWW-Authenticate", "Negotiate")
			w.Header().Add("WWW-Authenticate", "Digest")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	pr, err := ping(context.Background(), testRegistry, tprt)
	if err != nil {
		t.Errorf("ping() = %v", err)
	}
	if pr.challenge != "negotiate" {
		t.Errorf("ping(); got %v, want %v", pr.challenge, "negotiate")
	}
}

func TestUnsupportedStatus(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="http://auth.example.com/token`)
			http.Error(w, "Forbidden", http.StatusForbidden)
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	pr, err := ping(context.Background(), testRegistry, tprt)
	if err == nil {
		t.Errorf("ping() = %v", pr)
	}
}

func TestPingHttpFallback(t *testing.T) {
	tests := []struct {
		reg       name.Registry
		wantCount int64
		err       string
		contains  []string
	}{{
		reg:       mustRegistry("gcr.io"),
		wantCount: 1,
		err:       `Get "https://gcr.io/v2/": http: server gave HTTP response to HTTPS client`,
	}, {
		reg:       mustRegistry("ko.local"),
		wantCount: 2,
	}, {
		reg:       mustInsecureRegistry("us.gcr.io"),
		wantCount: 0,
		contains:  []string{"https://us.gcr.io/v2/", "http://us.gcr.io/v2/"},
	}}

	gotCount := int64(0)
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt64(&gotCount, 1)
			if r.URL.Scheme != "http" {
				// Sleep a little bit so we can exercise the
				// happy eyeballs race.
				time.Sleep(5 * time.Millisecond)
			}
			w.WriteHeader(http.StatusOK)
		}))
	defer server.Close()

	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	fallbackDelay = 2 * time.Millisecond

	for _, test := range tests {
		// This is the last one, fatal error it.
		if strings.Contains(test.reg.String(), "us.gcr.io") {
			server.Close()
		}

		_, err := ping(context.Background(), test.reg, tprt)
		if got, want := gotCount, test.wantCount; got != want {
			t.Errorf("%s: got %d requests, wanted %d", test.reg.String(), got, want)
		}
		gotCount = 0

		if err == nil {
			if test.err != "" {
				t.Error("expected err, got nil")
			}
			continue
		}
		if len(test.contains) != 0 {
			for _, c := range test.contains {
				if !strings.Contains(err.Error(), c) {
					t.Errorf("expected err to contain %q but did not: %q", c, err)
				}
			}
		} else if got, want := err.Error(), test.err; got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}

func mustRegistry(r string) name.Registry {
	reg, err := name.NewRegistry(r)
	if err != nil {
		panic(err)
	}
	return reg
}

func mustInsecureRegistry(r string) name.Registry {
	reg, err := name.NewRegistry(r, name.Insecure)
	if err != nil {
		panic(err)
	}
	return reg
}
