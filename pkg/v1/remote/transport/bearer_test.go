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
					fmt.Fprintf(w, `{%q: %q}`, tc.tokenKey, expectedToken)
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
				scheme:   "http",
			}

			if err := bt.refresh(context.Background()); (err != nil) != tc.wantErr {
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
			if r.URL.Path == "/v2/auth" {
				http.Redirect(w, r, "/redirect", http.StatusMovedPermanently)
				return
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

	client := http.Client{Transport: &bearerTransport{
		inner:    &http.Transport{},
		bearer:   authn.AuthConfig{RegistryToken: expectedToken},
		registry: registry,
		scheme:   "http",
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

func TestBearerTransportTokenRefresh(t *testing.T) {
	initialToken := "foo"
	refreshedToken := "bar"

	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hdr := r.Header.Get("Authorization")
			if hdr == "Bearer "+refreshedToken {
				w.WriteHeader(http.StatusOK)
				return
			}
			if strings.HasPrefix(hdr, "Basic ") {
				fmt.Fprintf(w, `{"token": %q}`, refreshedToken)
			}

			w.Header().Set("WWW-Authenticate", "scope=foo")
			w.WriteHeader(http.StatusUnauthorized)
		}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := name.NewRegistry(u.Host, name.WeakValidation)
	if err != nil {
		t.Fatalf("Unexpected error during NewRegistry: %v", err)
	}

	// Pass Username/Password
	transport := &bearerTransport{
		inner:    http.DefaultTransport,
		bearer:   authn.AuthConfig{RegistryToken: initialToken},
		basic:    &authn.Basic{Username: "foo", Password: "bar"},
		registry: registry,
		realm:    server.URL,
		scheme:   "http",
	}
	client := http.Client{Transport: transport}

	res, err := client.Get(fmt.Sprintf("http://%s/v2/foo/bar/blobs/blah", u.Host))
	if err != nil {
		t.Errorf("Unexpected error during client.Get: %v", err)
		return
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("client.Get final StatusCode got %v, want: %v", res.StatusCode, http.StatusOK)
	}
	if got, want := transport.bearer.RegistryToken, refreshedToken; got != want {
		t.Errorf("Expected Bearer token to be refreshed, got %v, want %v", got, want)
	}

	// Pass RegistryToken directly
	transport.bearer = authn.AuthConfig{RegistryToken: initialToken}
	transport.basic = &authn.Bearer{Token: refreshedToken}
	client = http.Client{Transport: transport}

	res, err = client.Get(fmt.Sprintf("http://%s/v2/foo/bar/blobs/blah", u.Host))
	if err != nil {
		t.Errorf("Unexpected error during client.Get: %v", err)
		return
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("client.Get final StatusCode got %v, want: %v", res.StatusCode, http.StatusOK)
	}
	if got, want := transport.bearer.RegistryToken, refreshedToken; got != want {
		t.Errorf("Expected Bearer token to be refreshed, got %v, want %v", got, want)
	}
}

func TestBearerTransportOauthRefresh(t *testing.T) {
	initialToken := "foo"
	accessToken := "bar"
	refreshToken := "baz"

	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				if it := r.FormValue("refresh_token"); it != initialToken {
					t.Errorf("want %s got %s", initialToken, it)
				}
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `{"access_token": %q, "refresh_token": %q}`, accessToken, refreshToken)
				return
			}

			hdr := r.Header.Get("Authorization")
			if hdr == "Bearer "+accessToken {
				w.WriteHeader(http.StatusOK)
				return
			}

			w.Header().Set("WWW-Authenticate", "scope=foo")
			w.WriteHeader(http.StatusUnauthorized)
		}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := name.NewRegistry(u.Host, name.WeakValidation)
	if err != nil {
		t.Errorf("Unexpected error during NewRegistry: %v", err)
	}

	transport := &bearerTransport{
		inner:    http.DefaultTransport,
		basic:    authn.FromConfig(authn.AuthConfig{IdentityToken: initialToken}),
		registry: registry,
		realm:    server.URL,
		scheme:   "http",
		scopes:   []string{"myscope"},
		service:  u.Host,
	}
	client := http.Client{Transport: transport}

	res, err := client.Get(fmt.Sprintf("http://%s/v2/foo/bar/blobs/blah", u.Host))
	if err != nil {
		t.Fatalf("Unexpected error during client.Get: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("client.Get final StatusCode got %v, want: %v", res.StatusCode, http.StatusOK)
	}
	if want, got := transport.bearer.RegistryToken, accessToken; want != got {
		t.Errorf("Expected Bearer token to be refreshed, got %v, want %v", got, want)
	}
	basicAuthConfig, err := transport.basic.Authorization()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := basicAuthConfig.IdentityToken, refreshToken; got != want {
		t.Errorf("Expected Basic IdentityToken to be refreshed, got %v, want %v", got, want)
	}
}

func TestBearerTransportOauth404Fallback(t *testing.T) {
	basicAuth := "basic_auth"
	identityToken := "identity_token"
	accessToken := "access_token"

	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusNotFound)
			}

			hdr := r.Header.Get("Authorization")
			if hdr == "Basic "+basicAuth {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `{"access_token": %q}`, accessToken)
			}
			if hdr == "Bearer "+accessToken {
				w.WriteHeader(http.StatusOK)
				return
			}

			w.Header().Set("WWW-Authenticate", "scope=foo")
			w.WriteHeader(http.StatusUnauthorized)
		}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := name.NewRegistry(u.Host, name.WeakValidation)
	if err != nil {
		t.Errorf("Unexpected error during NewRegistry: %v", err)
	}

	transport := &bearerTransport{
		inner: http.DefaultTransport,
		basic: authn.FromConfig(authn.AuthConfig{
			IdentityToken: identityToken,
			Auth:          basicAuth,
		}),
		registry: registry,
		realm:    server.URL,
		scheme:   "http",
		scopes:   []string{"myscope"},
		service:  u.Host,
	}
	client := http.Client{Transport: transport}

	res, err := client.Get(fmt.Sprintf("http://%s/v2/foo/bar/blobs/blah", u.Host))
	if err != nil {
		t.Fatalf("Unexpected error during client.Get: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("client.Get final StatusCode got %v, want: %v", res.StatusCode, http.StatusOK)
	}
	if got, want := transport.bearer.RegistryToken, accessToken; got != want {
		t.Errorf("Expected Bearer token to be refreshed, got %v, want %v", got, want)
	}
}

type recorder struct {
	reqs []*http.Request
	resp *http.Response
	err  error
}

func newRecorder(resp *http.Response, err error) *recorder {
	return &recorder{
		reqs: []*http.Request{},
		resp: resp,
		err:  err,
	}
}

func (r *recorder) RoundTrip(in *http.Request) (*http.Response, error) {
	r.reqs = append(r.reqs, in)
	return r.resp, r.err
}

func TestSchemeOverride(t *testing.T) {
	// Record the requests we get in the inner transport.
	cannedResponse := http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
	}
	rec := newRecorder(&cannedResponse, nil)
	registry, err := name.NewRegistry("example.com")
	if err != nil {
		t.Fatalf("Unexpected error during NewRegistry: %v", err)
	}
	st := &schemeTransport{
		inner:    rec,
		registry: registry,
		scheme:   "http",
	}

	// We should see the scheme be overridden to "http" for the registry, but the
	// scheme for the token server should be unchanged.
	tests := []struct {
		url        string
		wantScheme string
	}{{
		url:        "https://example.com",
		wantScheme: "http",
	}, {
		url:        "https://token.example.com",
		wantScheme: "https",
	}}

	for i, tt := range tests {
		req, err := http.NewRequest("GET", tt.url, nil)
		if err != nil {
			t.Fatalf("Unexpected error during NewRequest: %v", err)
		}

		if _, err := st.RoundTrip(req); err != nil {
			t.Fatalf("Unexpected error during RoundTrip: %v", err)
		}

		if got, want := rec.reqs[i].URL.Scheme, tt.wantScheme; got != want {
			t.Errorf("Wrong scheme: wanted %v, got %v", want, got)
		}
	}
}

func TestCanonicalAddressResolution(t *testing.T) {
	registry, err := name.NewRegistry("does-not-matter", name.WeakValidation)
	if err != nil {
		t.Errorf("Unexpected error during NewRegistry: %v", err)
	}

	tests := []struct {
		registry name.Registry
		scheme   string
		address  string
		want     string
	}{{
		registry: registry,
		scheme:   "http",
		address:  "registry.example.com",
		want:     "registry.example.com:80",
	}, {
		registry: registry,
		scheme:   "http",
		address:  "registry.example.com:12345",
		want:     "registry.example.com:12345",
	}, {
		registry: registry,
		scheme:   "https",
		address:  "registry.example.com",
		want:     "registry.example.com:443",
	}, {
		registry: registry,
		scheme:   "https",
		address:  "registry.example.com:12345",
		want:     "registry.example.com:12345",
	}, {
		registry: registry,
		scheme:   "http",
		address:  "registry.example.com:",
		want:     "registry.example.com:80",
	}, {
		registry: registry,
		scheme:   "https",
		address:  "registry.example.com:",
		want:     "registry.example.com:443",
	}, {
		registry: registry,
		scheme:   "http",
		address:  "2001:db8::1",
		want:     "[2001:db8::1]:80",
	}, {
		registry: registry,
		scheme:   "https",
		address:  "2001:db8::1",
		want:     "[2001:db8::1]:443",
	}, {
		registry: registry,
		scheme:   "http",
		address:  "[2001:db8::1]:12345",
		want:     "[2001:db8::1]:12345",
	}, {
		registry: registry,
		scheme:   "https",
		address:  "[2001:db8::1]:12345",
		want:     "[2001:db8::1]:12345",
	}, {
		registry: registry,
		scheme:   "http",
		address:  "[2001:db8::1]:",
		want:     "[2001:db8::1]:80",
	}, {
		registry: registry,
		scheme:   "https",
		address:  "[2001:db8::1]:",
		want:     "[2001:db8::1]:443",
	}, {
		registry: registry,
		scheme:   "https",
		address:  "something:is::wrong]:",
		want:     "something:is::wrong]:",
	}}

	for _, tt := range tests {
		got := canonicalAddress(tt.address, tt.scheme)
		if got != tt.want {
			t.Errorf("Wrong canonical host: wanted %v got %v", tt.want, got)
		}
	}
}

func TestInsufficientScope(t *testing.T) {
	wrong := "the-wrong-scope"
	right := "the-right-scope"
	realm := ""
	expectedService := "my-service.io"
	passed := false

	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()

			scopes := query["scope"]
			switch {
			case len(scopes) == 0:
				if !passed {
					w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer realm=%q,scope=%q", realm, right))
					w.WriteHeader(http.StatusUnauthorized)
				}
			case len(scopes) == 1:
				w.Write([]byte(`{"token": "arbitrary-token"}`))
			default:
				passed = true
				w.Write([]byte(`{"token": "arbitrary-token-2"}`))
			}
		}))
	defer server.Close()

	basic := &authn.Basic{Username: "foo", Password: "bar"}
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Error("Unexpected error during url.Parse: ", err)
	}
	realm = u.Host

	registry, err := name.NewRegistry(expectedService, name.WeakValidation)
	if err != nil {
		t.Error("Unexpected error during NewRegistry: ", err)
	}

	bt := &bearerTransport{
		inner:    http.DefaultTransport,
		basic:    basic,
		registry: registry,
		realm:    server.URL,
		scopes:   []string{wrong},
		service:  expectedService,
		scheme:   "http",
	}

	client := http.Client{Transport: bt}

	res, err := client.Get(fmt.Sprintf("http://%s/v2/foo/bar/blobs/blah", u.Host))
	if err != nil {
		t.Error("Unexpected error during client.Get: ", err)
		return
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("client.Get final StatusCode got %v, want: %v", res.StatusCode, http.StatusOK)
	}

	if !passed {
		t.Error("didn't refresh insufficient scope")
	}
}

// TestTokenServerRedirectSSRF verifies that a malicious token server cannot
// redirect token-fetch requests to private/loopback addresses, bypassing the
// initial validateRealmURL check.
func TestTokenServerRedirectSSRF(t *testing.T) {
	// internalServer simulates an internal service that should never be reached.
	internalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"token": "should-not-reach-this"}`)
	}))
	defer internalServer.Close()

	// tokenServer issues a redirect to the internalServer on every request.
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internalServer.URL+"/token", http.StatusFound)
	}))
	defer tokenServer.Close()

	registry, err := name.NewRegistry("registry.example.com", name.WeakValidation)
	if err != nil {
		t.Fatal(err)
	}

	bt := &bearerTransport{
		inner:    http.DefaultTransport,
		basic:    &authn.Basic{Username: "foo", Password: "bar"},
		registry: registry,
		realm:    tokenServer.URL + "/token",
		scopes:   []string{"repo:example/image:pull"},
		service:  "registry.example.com",
		scheme:   "http",
	}

	if err := bt.refresh(context.Background()); err == nil {
		t.Error("refresh() should have been rejected when token server redirects to loopback address")
	}
}

func TestValidateRealmURLUnspecified(t *testing.T) {
	// 0.0.0.0 and :: resolve to localhost on most OSes.
	// They should be blocked like other private addresses.
	tests := []struct {
		realm   string
		wantErr bool
	}{
		{"https://0.0.0.0/token", true},
		{"https://0.0.0.0:8443/token", true},
		{"https://[::]/token", true},
		{"https://[::]:8443/token", true},
		// existing checks still work
		{"https://127.0.0.1/token", true},
		{"https://[::1]/token", true},
		{"https://10.0.0.1/token", true},
		{"https://192.168.1.1/token", true},
		{"https://169.254.169.254/token", true},
		// public IPs are fine
		{"https://8.8.8.8/token", false},
		{"https://registry.example.com/token", false},
	}
	for _, tt := range tests {
		err := validateRealmURL(tt.realm, "", false)
		if tt.wantErr && err == nil {
			t.Errorf("validateRealmURL(%q) should have been rejected", tt.realm)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("validateRealmURL(%q) unexpected error: %v", tt.realm, err)
		}
	}
}

// TestValidateRealmURLSameHost verifies the same-host exception: a realm URL
// pointing at the same host the user is already talking to is allowed even
// when that host is private/loopback/link-local. See
// https://github.com/google/go-containerregistry/issues/2258.
func TestValidateRealmURLSameHost(t *testing.T) {
	tests := []struct {
		name         string
		realm        string
		registryHost string
		insecure     bool
		wantErr      bool
	}{
		{
			name:         "same loopback host:port allowed",
			realm:        "http://127.0.0.1:5000/auth",
			registryHost: "127.0.0.1:5000",
			insecure:     true,
		},
		{
			name:         "same loopback host but different port still blocked",
			realm:        "http://127.0.0.1:8080/auth",
			registryHost: "127.0.0.1:5000",
			insecure:     true,
			wantErr:      true,
		},
		{
			name:         "same private host (no port) allowed",
			realm:        "https://10.0.0.1/auth",
			registryHost: "10.0.0.1",
		},
		{
			name:         "same IPv6 loopback host:port allowed",
			realm:        "http://[::1]:5000/auth",
			registryHost: "[::1]:5000",
			insecure:     true,
		},
		{
			name:         "cross-host loopback redirect still blocked",
			realm:        "http://127.0.0.1:5000/auth",
			registryHost: "registry.example.com",
			insecure:     true,
			wantErr:      true,
		},
		{
			name:         "cross-host metadata redirect still blocked",
			realm:        "https://169.254.169.254/auth",
			registryHost: "registry.example.com",
			wantErr:      true,
		},
		{
			name:         "scheme check still applies on same host",
			realm:        "http://127.0.0.1:5000/auth",
			registryHost: "127.0.0.1:5000",
			insecure:     false,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRealmURL(tt.realm, tt.registryHost, tt.insecure)
			if tt.wantErr && err == nil {
				t.Errorf("validateRealmURL(%q, %q) should have returned an error", tt.realm, tt.registryHost)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateRealmURL(%q, %q) unexpected error: %v", tt.realm, tt.registryHost, err)
			}
		})
	}
}

// TestBearerTokenReappliedOnSameHostChallenge is the regression test for
// https://github.com/google/go-containerregistry/issues/2333: when the
// registry we authenticated against answers with a 401 Bearer challenge
// (e.g. token expiry mid-session), the freshly refreshed token must be
// applied to the retry so it succeeds rather than failing with a second 401.
func TestBearerTokenReappliedOnSameHostChallenge(t *testing.T) {
	const (
		staleToken = "stale-token"
		freshToken = "fresh-token"
	)

	attempts := 0
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Header.Get("Authorization") == "Bearer "+freshToken {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="https://token.example.com/token",service="registry"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer registryServer.Close()

	// bt.registry IS the server's host, so matchesHost is true: this is the
	// registry we authenticated against, and re-attaching the token is safe.
	u, err := url.Parse(registryServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := name.NewRegistry(u.Host, name.WeakValidation)
	if err != nil {
		t.Fatal(err)
	}

	bt := &bearerTransport{
		inner:  http.DefaultTransport,
		bearer: authn.AuthConfig{RegistryToken: staleToken},
		// authn.Bearer causes refresh() to set bearer.RegistryToken directly from
		// the credential without a network call, so no SSRF guard is triggered.
		basic:    &authn.Bearer{Token: freshToken},
		registry: registry,
		realm:    "https://token.example.com/token",
		scopes:   []string{"repo:example/image:pull"},
		service:  "registry",
		scheme:   "http",
	}

	res, err := (&http.Client{Transport: bt}).Get(registryServer.URL + "/v2/example/image/manifests/latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d: fresh bearer token was not applied on same-host retry", res.StatusCode, http.StatusOK)
	}
	if got := bt.bearer.RegistryToken; got != freshToken {
		t.Errorf("bearer.RegistryToken = %q, want %q", got, freshToken)
	}
	if attempts != 2 {
		t.Errorf("registry received %d request(s), want 2 (401 then 200)", attempts)
	}
}

// TestBearerTokenNotLeakedOnCrossHostChallenge verifies that when a request
// has been redirected to a host that differs from the registry we logged in
// to, the refreshed token is NOT re-attached. A malicious or compromised
// registry must not be able to 302 the request to an attacker host, answer
// with a Bearer challenge, and harvest the operator's registry credential.
func TestBearerTokenNotLeakedOnCrossHostChallenge(t *testing.T) {
	const (
		staleToken = "stale-token"
		freshToken = "fresh-token"
	)

	var gotAuth string
	// attackerServer is NOT bt.registry. It issues a Bearer challenge and
	// records any Authorization header it receives.
	attackerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a := r.Header.Get("Authorization"); a != "" {
			gotAuth = a
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="https://token.example.com/token",service="attacker"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer attackerServer.Close()

	registry, err := name.NewRegistry("registry.example.com", name.WeakValidation)
	if err != nil {
		t.Fatal(err)
	}

	bt := &bearerTransport{
		inner:    http.DefaultTransport,
		bearer:   authn.AuthConfig{RegistryToken: staleToken},
		basic:    &authn.Bearer{Token: freshToken},
		registry: registry, // does NOT match attackerServer's host
		realm:    "https://token.example.com/token",
		scopes:   []string{"repo:example/image:pull"},
		service:  "registry.example.com",
		scheme:   "http",
	}

	res, err := (&http.Client{Transport: bt}).Get(attackerServer.URL + "/v2/example/image/manifests/latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer res.Body.Close()

	if gotAuth != "" {
		t.Fatalf("credential leaked to cross-host challenger: Authorization=%q", gotAuth)
	}
}
