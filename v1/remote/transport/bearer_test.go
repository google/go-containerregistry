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

	"github.com/google/go-containerregistry/authn"
)

func TestBearerRefresh(t *testing.T) {
	expectedToken := "Sup3rDup3rS3cr3tz"
	expectedScope := "this-is-your-scope"
	expectedService := "my-service.io"
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
			w.Write([]byte(fmt.Sprintf(`{"token": %q}`, expectedToken)))
		}))
	defer server.Close()

	basic := &authn.Basic{Username: "foo", Password: "bar"}

	bt := &bearerTransport{
		inner:   http.DefaultTransport,
		basic:   basic,
		realm:   server.URL,
		scope:   expectedScope,
		service: expectedService,
	}

	if err := bt.refresh(); err != nil {
		t.Errorf("refresh() = %v", err)
	}
}

func TestBearerTransport(t *testing.T) {
	expectedToken := "sdkjhfskjdhfkjshdf"
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got, want := r.Header.Get("Authorization"), "Bearer "+expectedToken; got != want {
				t.Errorf("Header.Get(Authorization); got %v, want %v", got, want)
			}
			w.WriteHeader(http.StatusOK)
		}))
	defer server.Close()

	inner := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	bearer := &authn.Bearer{Token: expectedToken}
	client := http.Client{Transport: &bearerTransport{
		inner:  inner,
		bearer: bearer,
	}}

	_, err := client.Get("http://gcr.io/v2/auth")
	if err != nil {
		t.Errorf("Unexpected error during Get: %v", err)
	}
}

// TODO(mattmoor): 401 response prompts a refresh (NYI)
