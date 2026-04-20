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

package remote

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

func TestDelete(t *testing.T) {
	expectedRepo := "write/time"
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case manifestPath:
			if r.Method != http.MethodDelete {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodDelete)
			}
			http.Error(w, "Deleted", http.StatusOK)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}
	tag, err := name.NewTag(fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo), name.WeakValidation)
	if err != nil {
		t.Fatalf("NewTag() = %v", err)
	}

	if err := Delete(tag); err != nil {
		t.Errorf("Delete() = %v", err)
	}
}

func TestDeleteBadStatus(t *testing.T) {
	expectedRepo := "write/time"
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case manifestPath:
			if r.Method != http.MethodDelete {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodDelete)
			}
			http.Error(w, "Boom Goes Server", http.StatusInternalServerError)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}
	tag, err := name.NewTag(fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo), name.WeakValidation)
	if err != nil {
		t.Fatalf("NewTag() = %v", err)
	}

	if err := Delete(tag); err == nil {
		t.Error("Delete() = nil; wanted error")
	}
}

// TestDeleteRequestsDeleteScope verifies that Delete asks the token endpoint
// for a scope that includes the "delete" action. Registries such as IBM Cloud
// Container Registry require the explicit "delete" action to be requested in
// the Bearer token scope.
func TestDeleteRequestsDeleteScope(t *testing.T) {
	// Sanity-check the DeleteScope constant itself.
	if !strings.Contains(transport.DeleteScope, "delete") {
		t.Errorf("DeleteScope = %q; want it to include \"delete\"", transport.DeleteScope)
	}

	const (
		host         = "delete-scope-test.example.com"
		expectedRepo = "write/time"
		tokenPath    = "/v2/token"
	)
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)

	var gotScopes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == tokenPath:
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm() = %v", err)
			}
			gotScopes = r.Form["scope"]
			w.Write([]byte(`{"token": "mytoken"}`))
		case r.URL.Path == "/v2/" && r.Header.Get("Authorization") == "":
			// Initial ping: challenge for a Bearer token pointing at the token endpoint.
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="http://%s%s",service="fake"`, host, tokenPath))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == manifestPath:
			if r.Method != http.MethodDelete {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodDelete)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer srv.Close()

	// Redirect every TCP connection to the test server so we can use a
	// non-loopback host name in the tag and realm (bearer transport rejects
	// realms that resolve to loopback/private addresses).
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", srv.URL, err)
	}
	tprt := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, u.Host)
		},
	}

	tag, err := name.NewTag(fmt.Sprintf("%s/%s:latest", host, expectedRepo), name.WeakValidation, name.Insecure)
	if err != nil {
		t.Fatalf("NewTag() = %v", err)
	}

	if err := Delete(tag, WithAuth(authn.Anonymous), WithTransport(tprt)); err != nil {
		t.Errorf("Delete() = %v", err)
	}

	if len(gotScopes) == 0 {
		t.Fatalf("token endpoint was not called; scopes = %v", gotScopes)
	}
	var found bool
	for _, s := range gotScopes {
		if strings.Contains(s, "delete") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("token request scopes = %v; want one to contain \"delete\"", gotScopes)
	}
}
