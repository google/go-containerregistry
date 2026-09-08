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

package ocir

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/oracle/oci-go-sdk/v65/common"
)

func testProvider(t *testing.T) common.ConfigurationProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return common.NewRawConfigurationProvider(
		"ocid1.tenancy.oc1..testtenancy",
		"ocid1.user.oc1..testuser",
		"us-ashburn-1",
		"aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
		string(pemKey),
		nil)
}

func staticProviders(p ...common.ConfigurationProvider) func() []common.ConfigurationProvider {
	return func() []common.ConfigurationProvider { return p }
}

// fakeJWT builds an unsigned JWT-shaped token whose sub claim carries the
// given userId, mirroring OCIR's nested-JSON sub format.
func fakeJWT(t *testing.T, userID string) string {
	t.Helper()
	sub, err := json.Marshal(map[string]any{"userId": userID, "tenantId": "tenant", "claims": []string{}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]string{"sub": string(sub)})
	if err != nil {
		t.Fatal(err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func TestGet(t *testing.T) {
	var gotReq *http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"token":"test-jwt"}`))
	}))
	defer ts.Close()

	h := &Helper{
		client:    ts.Client(),
		endpoint:  func(host string) string { return ts.URL + tokenPath },
		providers: staticProviders(testProvider(t)),
	}

	username, password, err := h.Get("iad.ocir.io")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if username != "BEARER_TOKEN" {
		t.Errorf("username: got %q, want %q", username, "BEARER_TOKEN")
	}
	if password != "test-jwt" {
		t.Errorf("password: got %q, want %q", password, "test-jwt")
	}

	if gotReq == nil {
		t.Fatal("token endpoint was not called")
	}
	if gotReq.Method != http.MethodGet {
		t.Errorf("method: got %q, want GET", gotReq.Method)
	}
	if gotReq.URL.Path != tokenPath {
		t.Errorf("path: got %q, want %q", gotReq.URL.Path, tokenPath)
	}
	if auth := gotReq.Header.Get("Authorization"); !strings.HasPrefix(auth, "Signature ") {
		t.Errorf("Authorization: got %q, want OCI request signature", auth)
	}
	if gotReq.Header.Get("Date") == "" {
		t.Error("Date header not set on signed request")
	}
}

func TestGetNonOCIRHost(t *testing.T) {
	// The host check must reject non-OCIR registries before any principal
	// resolution or network access happens, so exercise the default Helper.
	h := NewHelper()
	for _, serverURL := range []string{
		"index.docker.io",
		"gcr.io",
		"https://registry.example.com/v2/",
		"ocir.io",
		"evil-ocir.io",
		"iad.ocir.io.example.com",
	} {
		_, _, err := h.Get(serverURL)
		if !credentials.IsErrCredentialsNotFound(err) {
			t.Errorf("Get(%q): got %v, want ErrCredentialsNotFound", serverURL, err)
		}
	}
}

func TestGetNoPrincipal(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	// No providers at all.
	h := &Helper{
		client:    ts.Client(),
		endpoint:  func(host string) string { return ts.URL + tokenPath },
		providers: staticProviders(),
	}
	if _, _, err := h.Get("iad.ocir.io"); !credentials.IsErrCredentialsNotFound(err) {
		t.Errorf("Get with no providers: got %v, want ErrCredentialsNotFound", err)
	}

	// A provider that cannot produce a signing key, like
	// common.DefaultConfigProvider without any OCI configuration.
	h.providers = staticProviders(common.NewRawConfigurationProvider("", "", "", "", "", nil))
	if _, _, err := h.Get("iad.ocir.io"); !credentials.IsErrCredentialsNotFound(err) {
		t.Errorf("Get with unusable provider: got %v, want ErrCredentialsNotFound", err)
	}

	if called {
		t.Error("token endpoint was called without a usable principal")
	}
}

func TestGetTokenEndpointErrors(t *testing.T) {
	anonJWT := fakeJWT(t, "anon-f82f6dd96e37fdb72ad")
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"unauthorized", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}},
		{"empty token", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{}`))
		}},
		{"malformed body", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`not json`))
		}},
		{"anonymous token", func(w http.ResponseWriter, r *http.Request) {
			// OCIR returns 200 with an anonymous token instead of
			// rejecting an invalid request signature.
			body, _ := json.Marshal(map[string]string{"token": anonJWT})
			w.Write(body)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(tc.handler)
			defer ts.Close()
			h := &Helper{
				client:    ts.Client(),
				endpoint:  func(host string) string { return ts.URL + tokenPath },
				providers: staticProviders(testProvider(t)),
			}
			if _, _, err := h.Get("iad.ocir.io"); !credentials.IsErrCredentialsNotFound(err) {
				t.Errorf("Get: got %v, want ErrCredentialsNotFound", err)
			}
		})
	}
}

func TestGetPrincipalToken(t *testing.T) {
	// A token issued to a real principal must be accepted.
	userJWT := fakeJWT(t, "ocid1.user.oc1..sometestuser")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(map[string]string{"token": userJWT})
		w.Write(body)
	}))
	defer ts.Close()
	h := &Helper{
		client:    ts.Client(),
		endpoint:  func(host string) string { return ts.URL + tokenPath },
		providers: staticProviders(testProvider(t)),
	}
	_, password, err := h.Get("iad.ocir.io")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if password != userJWT {
		t.Errorf("password: got %q, want %q", password, userJWT)
	}
}

func TestAnonymousToken(t *testing.T) {
	for _, tc := range []struct {
		name  string
		token string
		want  bool
	}{
		{"anonymous", fakeJWT(t, "anon-f82f6dd96e37fdb72ad"), true},
		{"real user", fakeJWT(t, "ocid1.user.oc1..sometestuser"), false},
		// Fail open on anything that doesn't match the expected shape.
		{"not a JWT", "opaque-token", false},
		{"bad base64", "a.!!!.c", false},
		{"non-JSON payload", "a." + base64.RawURLEncoding.EncodeToString([]byte("hi")) + ".c", false},
		{"non-JSON sub", "a." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"plain-string"}`)) + ".c", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := anonymousToken(tc.token); got != tc.want {
				t.Errorf("anonymousToken: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRegistryHost(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"iad.ocir.io", "iad.ocir.io"},
		{"us-ashburn-1.ocir.io", "us-ashburn-1.ocir.io"},
		{"https://iad.ocir.io", "iad.ocir.io"},
		{"https://iad.ocir.io/v2/", "iad.ocir.io"},
		{"iad.ocir.io:443", "iad.ocir.io"},
		{"iad.ocir.io/some/repo", "iad.ocir.io"},
		{"index.docker.io", "index.docker.io"},
	} {
		if got := registryHost(tc.in); got != tc.want {
			t.Errorf("registryHost(%q): got %q, want %q", tc.in, got, tc.want)
		}
	}
}
