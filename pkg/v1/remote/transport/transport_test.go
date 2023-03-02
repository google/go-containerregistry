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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

var (
	testReference, _ = name.NewTag("localhost:8080/user/image:latest", name.StrictValidation)
)

func TestTransportNoActionIfTransportIsAlreadyWrapper(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="http://foo.io"`)
			http.Error(w, "Should not contact the server", http.StatusBadRequest)
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	wTprt := &Wrapper{inner: tprt}

	if _, err := NewWithContext(context.Background(), testReference.Context().Registry, nil, wTprt, []string{testReference.Scope(PullScope)}); err != nil {
		t.Errorf("NewWithContext unexpected error %s", err)
	}
}

func TestTransportSelectionAnonymous(t *testing.T) {
	// Record the requests we get in the inner transport.
	cannedResponse := http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("")),
	}
	recorder := newRecorder(&cannedResponse, nil)

	basic := &authn.Basic{Username: "foo", Password: "bar"}
	reg := testReference.Context().Registry

	tp, err := NewWithContext(context.Background(), reg, basic, recorder, []string{testReference.Scope(PullScope)})
	if err != nil {
		t.Errorf("NewWithContext() = %v", err)
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/v2/anything", reg), nil)
	if err != nil {
		t.Fatalf("Unexpected error during NewRequest: %v", err)
	}
	if _, err := tp.RoundTrip(req); err != nil {
		t.Fatalf("Unexpected error during RoundTrip: %v", err)
	}

	if got, want := len(recorder.reqs), 2; got != want {
		t.Fatalf("expected %d requests, got %d", want, got)
	}
	recorded := recorder.reqs[1]
	if got, want := recorded.URL.Scheme, "https"; got != want {
		t.Errorf("wrong scheme, want %s got %s", want, got)
	}
}

func TestTransportSelectionBasic(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("WWW-Authenticate", `Basic`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	basic := &authn.Basic{Username: "foo", Password: "bar"}

	tp, err := NewWithContext(context.Background(), testReference.Context().Registry, basic, tprt, []string{testReference.Scope(PullScope)})
	if err != nil {
		t.Errorf("NewWithContext() = %v", err)
	}
	if tpw, ok := tp.(*Wrapper); !ok {
		t.Errorf("NewWithContext(); got %T, want *Wrapper", tp)
	} else if _, ok := tpw.inner.(*basicTransport); !ok {
		t.Errorf("NewWithContext(); got %T, want *basicTransport", tp)
	}
}

type badAuth struct{}

func (a *badAuth) Authorization() (*authn.AuthConfig, error) {
	return nil, errors.New("sorry dave, I'm afraid I can't let you do that")
}

func TestTransportBadAuth(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="http://foo.io"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	if _, err := NewWithContext(context.Background(), testReference.Context().Registry, &badAuth{}, tprt, []string{testReference.Scope(PullScope)}); err == nil {
		t.Errorf("NewWithContext() expected err, got nil")
	}
}

func TestTransportSelectionBearer(t *testing.T) {
	request := 0
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			request++
			switch request {
			case 1:
				// This is an https request that fails, causing us to fall back to http.
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			case 2:
				w.Header().Set("WWW-Authenticate", `Bearer realm="http://foo.io"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			case 3:
				hdr := r.Header.Get("Authorization")
				if !strings.HasPrefix(hdr, "Basic ") {
					t.Errorf("Header.Get(Authorization); got %v, want Basic prefix", hdr)
				}
				if got, want := r.FormValue("scope"), testReference.Scope(PullScope); got != want {
					t.Errorf("FormValue(scope); got %v, want %v", got, want)
				}
				// Check that the service isn't set (we didn't specify it above)
				// https://github.com/google/go-containerregistry/issues/1359
				if got, want := r.FormValue("service"), ""; got != want {
					t.Errorf("FormValue(service); got %q, want %q", got, want)
				}
				w.Write([]byte(`{"token": "dfskdjhfkhsjdhfkjhsdf"}`))
			}
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	basic := &authn.Basic{Username: "foo", Password: "bar"}
	tp, err := NewWithContext(context.Background(), testReference.Context().Registry, basic, tprt, []string{testReference.Scope(PullScope)})
	if err != nil {
		t.Errorf("NewWithContext() = %v", err)
	}
	if tpw, ok := tp.(*Wrapper); !ok {
		t.Errorf("NewWithContext(); got %T, want *Wrapper", tp)
	} else if _, ok := tpw.inner.(*bearerTransport); !ok {
		t.Errorf("NewWithContext(); got %T, want *bearerTransport", tp)
	}
}

func TestTransportSelectionBearerMissingRealm(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("WWW-Authenticate", `Bearer service="gcr.io"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	basic := &authn.Basic{Username: "foo", Password: "bar"}
	tp, err := NewWithContext(context.Background(), testReference.Context().Registry, basic, tprt, []string{testReference.Scope(PullScope)})
	if err == nil || !strings.Contains(err.Error(), "missing realm") {
		t.Errorf("NewWithContext() = %v, %v", tp, err)
	}
}

func TestTransportSelectionBearerAuthError(t *testing.T) {
	request := 0
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			request++
			switch request {
			case 1:
				w.Header().Set("WWW-Authenticate", `Bearer realm="http://foo.io"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			case 2:
				http.Error(w, "Oops", http.StatusInternalServerError)
			}
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	basic := &authn.Basic{Username: "foo", Password: "bar"}
	tp, err := NewWithContext(context.Background(), testReference.Context().Registry, basic, tprt, []string{testReference.Scope(PullScope)})
	if err == nil {
		t.Errorf("NewWithContext() = %v", tp)
	}
}

func TestTransportSelectionUnrecognizedChallenge(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("WWW-Authenticate", `Unrecognized`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}))
	defer server.Close()
	tprt := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	basic := &authn.Basic{Username: "foo", Password: "bar"}
	tp, err := NewWithContext(context.Background(), testReference.Context().Registry, basic, tprt, []string{testReference.Scope(PullScope)})
	if err == nil || !strings.Contains(err.Error(), "challenge") {
		t.Errorf("NewWithContext() = %v, %v", tp, err)
	}
}

func TestTransportAlwaysTriesHttps(t *testing.T) {
	// Use a NewTLSServer so that this speaks TLS even though it's localhost.
	// This ensures that we try https even for local registries.
	count := 0
	server := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count++
			w.Write([]byte(`{"token": "dfskdjhfkhsjdhfkjhsdf"}`))
		}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Errorf("Unexpected error during url.Parse: %v", err)
	}
	registry, err := name.NewRegistry(u.Host, name.WeakValidation)
	if err != nil {
		t.Errorf("Unexpected error during NewRegistry: %v", err)
	}

	basic := &authn.Basic{Username: "foo", Password: "bar"}
	tp, err := NewWithContext(context.Background(), registry, basic, server.Client().Transport, []string{testReference.Scope(PullScope)})
	if err != nil {
		t.Fatalf("NewWithContext() = %v, %v", tp, err)
	}
	if count == 0 {
		t.Errorf("failed to call TLS localhost server")
	}
}
