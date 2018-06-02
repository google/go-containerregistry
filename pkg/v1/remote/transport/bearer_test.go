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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

func TestBearerRefresh(t *testing.T) {
	expectedToken := "Sup3rDup3rS3cr3tz"
	expectedScope := "this-is-your-scope"
	expectedService := "my-service.io"

	cases := []struct {
		tokenKey string
		wantErr  bool
	}{{
		tokenKey: "token",
		wantErr:  false,
	}, {
		tokenKey: "access_token",
		wantErr:  false,
	}, {
		tokenKey: "tolkien",
		wantErr:  true,
	}}

	for _, tc := range cases {
		t.Run(tc.tokenKey, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hdr := r.Header.Get("Authorization")
					if !strings.HasPrefix(hdr, "Basic ") {
						t.Errorf("Header.Get(Authorization); got %v, want Basic prefix", hdr)
					}
					if got, want := r.FormValue("scope"), expectedScope; got != want {
						t.Errorf("FormValue(scope); got %v, want %v", got, want)
					}
					if got, want := r.FormValue("service"), expectedService; got != want {
						t.Errorf("FormValue(service); got %v, want %v", got, want)
					}
					w.Write([]byte(fmt.Sprintf(`{%q: %q}`, tc.tokenKey, expectedToken)))
				}))
			defer server.Close()

			basic := &authn.Basic{Username: "foo", Password: "bar"}
			registry, err := name.NewRegistry(expectedService, name.WeakValidation)
			if err != nil {
				t.Errorf("Unexpected error during NewRegistry: %v", err)
			}

			bt := &bearerTransport{
				inner:    http.DefaultTransport,
				basic:    basic,
				registry: registry,
				realm:    server.URL,
				scopes:   []string{expectedScope},
				service:  expectedService,
			}

			if err := bt.refresh(); (err != nil) != tc.wantErr {
				t.Errorf("refresh() = %v", err)
			}
		})
	}
}

func TestBearerTransport(t *testing.T) {
	expectedToken := "sdkjhfskjdhfkjshdf"

	blobServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// We don't expect the blobServer to receive bearer tokens.
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("Header.Get(Authorization); got %v, want empty string", got)
			}
			w.WriteHeader(http.StatusOK)
		}))
	defer blobServer.Close()

	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got, want := r.Header.Get("Authorization"), "Bearer "+expectedToken; got != want {
				t.Errorf("Header.Get(Authorization); got %v, want %v", got, want)
			}

			if strings.Contains(r.URL.Path, "blobs") {
				http.Redirect(w, r, blobServer.URL, http.StatusFound)
				return
			}
			w.WriteHeader(http.StatusOK)
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

	bearer := &authn.Bearer{Token: expectedToken}
	client := http.Client{Transport: &bearerTransport{
		inner:    &http.Transport{},
		bearer:   bearer,
		registry: registry,
	}}

	_, err = client.Get(fmt.Sprintf("http://%s/v2/auth", u.Host))
	if err != nil {
		t.Errorf("Unexpected error during Get: %v", err)
	}

	_, err = client.Get(fmt.Sprintf("http://%s/v2/foo/bar/blobs/blah", u.Host))
	if err != nil {
		t.Errorf("Unexpected error during Get: %v", err)
	}
}

// TODO(mattmoor): 401 response prompts a refresh (NYI)
