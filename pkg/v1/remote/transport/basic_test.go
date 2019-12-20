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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
)

func TestBasicTransport(t *testing.T) {
	username := "foo"
	password := "bar"
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hdr := r.Header.Get("Authorization")
			if !strings.HasPrefix(hdr, "Basic ") {
				t.Errorf("Header.Get(Authorization); got %v, want Basic prefix", hdr)
			}
			user, pass, _ := r.BasicAuth()
			if user != username || pass != password {
				t.Error("Invalid credentials.")
			}
			if r.URL.Path == "/v2/auth" {
				http.Redirect(w, r, "/redirect", http.StatusMovedPermanently)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
	defer server.Close()

	inner := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	basic := &authn.Basic{Username: username, Password: password}
	client := http.Client{Transport: &basicTransport{inner: inner, auth: basic, target: "gcr.io"}}

	_, err := client.Get("http://gcr.io/v2/auth")
	if err != nil {
		t.Errorf("Unexpected error during Get: %v", err)
	}
}

func TestBasicTransportRegistryToken(t *testing.T) {
	token := "mytoken"
	for _, tc := range []struct {
		auth    authn.Authenticator
		hdr     string
		wantErr bool
	}{{
		auth: authn.FromConfig(authn.AuthConfig{RegistryToken: token}),
		hdr:  "Bearer mytoken",
	}, {
		auth: authn.FromConfig(authn.AuthConfig{Auth: token}),
		hdr:  "Basic mytoken",
	}, {
		auth: authn.Anonymous,
		hdr:  "",
	}, {
		auth:    &badAuth{},
		hdr:     "",
		wantErr: true,
	}} {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hdr := r.Header.Get("Authorization")
				want := tc.hdr
				if hdr != want {
					t.Errorf("Header.Get(Authorization); got %v, want %s", hdr, want)
				}
				if r.URL.Path == "/v2/auth" {
					http.Redirect(w, r, "/redirect", http.StatusMovedPermanently)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
		defer server.Close()

		inner := &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(server.URL)
			},
		}

		client := http.Client{Transport: &basicTransport{inner: inner, auth: tc.auth, target: "gcr.io"}}

		_, err := client.Get("http://gcr.io/v2/auth")
		if err != nil && !tc.wantErr {
			t.Errorf("Unexpected error during Get: %v", err)
		}
	}
}

func TestBasicTransportWithEmptyAuthnCred(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c, ok := r.Header["Authorization"]; ok && c[0] == "" {
				t.Error("got empty Authorization header")
			}
			if r.URL.Path == "/v2/auth" {
				http.Redirect(w, r, "/redirect", http.StatusMovedPermanently)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
	defer server.Close()

	inner := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	client := http.Client{Transport: &basicTransport{inner: inner, auth: authn.Anonymous, target: "gcr.io"}}
	_, err := client.Get("http://gcr.io/v2/auth")
	if err != nil {
		t.Errorf("Unexpected error during Get: %v", err)
	}
}
