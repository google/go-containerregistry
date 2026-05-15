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

package remote

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestCheckRedirectSSRF(t *testing.T) {
	// makeReq builds a redirect target request (dest) with a non-nil Response
	// and one prior request (origHost) so the guard logic actually runs.
	makeReq := func(dest, origHost string) (*http.Request, []*http.Request) {
		u, _ := url.Parse(dest)
		req := &http.Request{
			URL:      u,
			Response: &http.Response{},
		}
		via := []*http.Request{{URL: &url.URL{Host: origHost}}}
		return req, via
	}

	// Cross-host redirects to private/link-local/loopback IPs must be blocked.
	blocked := []struct{ orig, dest string }{
		{"registry.example.com", "http://169.254.169.254/latest/meta-data/"}, // AWS/GCP IMDS link-local
		{"registry.example.com", "http://192.168.1.1/admin"},                 // RFC 1918 private
		{"registry.example.com", "http://10.0.0.1/internal"},                 // RFC 1918 private
		{"registry.example.com", "http://172.16.0.1/secret"},                 // RFC 1918 private
		{"registry.example.com", "http://127.0.0.1:9999/creds"},              // loopback
		{"registry.example.com", "http://0.0.0.0/anything"},                  // unspecified
		{"registry.example.com", "http://[fd00:ec2::254]/latest/meta-data/"}, // IPv6 IMDS
		{"registry.example.com", "http://[::1]/internal"},                    // IPv6 loopback
	}
	for _, tc := range blocked {
		req, via := makeReq(tc.dest, tc.orig)
		if err := checkRedirectSSRF(req, via); err == nil {
			t.Errorf("checkRedirectSSRF(orig=%q, dest=%q): expected SSRF error, got nil", tc.orig, tc.dest)
		}
	}

	// Same-host redirects must be allowed even when the host is a loopback
	// address (e.g. local test registries on 127.0.0.1).
	sameHost := []struct{ orig, dest string }{
		{"registry.example.com", "https://registry.example.com/v2/foo/bar/blobs/uploads/1"},
		{"127.0.0.1:5000", "http://127.0.0.1:5000/v2/foo/bar/blobs/uploads/1"},
		{"[::1]:5000", "http://[::1]:5000/v2/foo/bar/blobs/uploads/1"},
	}
	for _, tc := range sameHost {
		req, via := makeReq(tc.dest, tc.orig)
		if err := checkRedirectSSRF(req, via); err != nil {
			t.Errorf("checkRedirectSSRF(orig=%q, dest=%q): unexpected error: %v", tc.orig, tc.dest, err)
		}
	}

	// Cross-host redirect to a public IP must be allowed.
	req, via := makeReq("http://8.8.8.8/layer.tar.gz", "registry.example.com")
	if err := checkRedirectSSRF(req, via); err != nil {
		t.Errorf("checkRedirectSSRF to public IP: unexpected error: %v", err)
	}

	// Cross-host redirect to a DNS hostname (not an IP literal) must be allowed.
	req, via = makeReq("https://storage.googleapis.com/bucket/layer.tar.gz", "registry.example.com")
	if err := checkRedirectSSRF(req, via); err != nil {
		t.Errorf("checkRedirectSSRF to DNS hostname: unexpected error: %v", err)
	}

	// len(via)==0 → initial request, not a redirect → always allowed.
	req = &http.Request{
		URL:      &url.URL{Host: "169.254.169.254"},
		Response: &http.Response{},
	}
	if err := checkRedirectSSRF(req, nil); err != nil {
		t.Errorf("checkRedirectSSRF with empty via: expected nil, got %v", err)
	}

	// req.Response==nil → no redirect response to inspect → always allowed.
	req = &http.Request{URL: &url.URL{Host: "169.254.169.254"}}
	via = []*http.Request{{URL: &url.URL{Host: "registry.example.com"}}}
	if err := checkRedirectSSRF(req, via); err != nil {
		t.Errorf("checkRedirectSSRF with nil Response: expected nil, got %v", err)
	}
}

func TestValidateForeignURL(t *testing.T) {
	tests := []struct {
		url      string
		insecure bool
		wantErr  bool
	}{
		// public HTTPS — always allowed
		{"https://cdn.example.com/layer.tar.gz", false, false},
		{"https://8.8.8.8/layer.tar.gz", false, false},
		// public HTTP — only for insecure registries
		{"http://cdn.example.com/layer.tar.gz", true, false},
		{"http://cdn.example.com/layer.tar.gz", false, true},
		// loopback — rejected even for insecure
		{"http://127.0.0.1/layer.tar.gz", true, true},
		{"https://127.0.0.1/layer.tar.gz", false, true},
		{"https://[::1]/layer.tar.gz", false, true},
		// link-local
		{"https://169.254.169.254/latest/meta-data/", false, true},
		{"http://169.254.169.254/latest/meta-data/", true, true},
		// private RFC 1918
		{"https://10.0.0.1/layer.tar.gz", false, true},
		{"https://192.168.1.1/layer.tar.gz", false, true},
		// unspecified
		{"https://0.0.0.0/layer.tar.gz", false, true},
		// disallowed schemes
		{"ftp://cdn.example.com/layer.tar.gz", false, true},
		{"file:///etc/passwd", false, true},
	}
	for _, tt := range tests {
		err := validateForeignURL(tt.url, tt.insecure)
		if tt.wantErr && err == nil {
			t.Errorf("validateForeignURL(%q, insecure=%v) should have been rejected", tt.url, tt.insecure)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("validateForeignURL(%q, insecure=%v) unexpected error: %v", tt.url, tt.insecure, err)
		}
	}
}

// TestPullingForeignLayerSSRF verifies that a manifest whose foreign-layer URL
// points to a private or loopback address is rejected.
func TestPullingForeignLayerSSRF(t *testing.T) {
	img := randomImage(t)
	expectedRepo := "foo/bar"

	foreignLayer, err := random.Layer(1024, types.DockerForeignLayer)
	if err != nil {
		t.Fatal(err)
	}

	img, err = mutate.Append(img, mutate.Addendum{
		Layer: foreignLayer,
		URLs:  []string{"http://169.254.169.254/latest/meta-data/iam/security-credentials/"},
	})
	if err != nil {
		t.Fatal(err)
	}

	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)
	configPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, mustConfigName(t, img))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case configPath:
			w.Write(mustRawConfigFile(t, img))
		case manifestPath:
			w.Write(mustRawManifest(t, img))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	tag := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo))
	rmt, err := Image(tag, WithTransport(http.DefaultTransport))
	if err != nil {
		t.Fatal(err)
	}

	layers, err := rmt.Layers()
	if err != nil {
		t.Fatal(err)
	}
	_, err = layers[1].Compressed()
	if err == nil {
		t.Error("Compressed() should have been rejected for a foreign layer URL pointing to a private address")
	}
}

// TestPullingForeignLayerSSRFViaRedirect verifies the CheckRedirect hook in
// fetchForeignBlobURL rejects redirects to private/loopback addresses.
func TestPullingForeignLayerSSRFViaRedirect(t *testing.T) {
	img := randomImage(t)
	expectedRepo := "foo/bar"

	foreignLayer, err := random.Layer(1024, types.DockerForeignLayer)
	if err != nil {
		t.Fatal(err)
	}

	victim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"AccessKeyId":"ASIA_LEAKED","SecretAccessKey":"LEAKED_SECRET"}`))
	}))
	defer victim.Close()

	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, victim.URL+"/credentials", http.StatusFound)
	}))
	defer attacker.Close()

	img, err = mutate.Append(img, mutate.Addendum{
		Layer: foreignLayer,
		URLs:  []string{attacker.URL + "/layer.tar.gz"},
	})
	if err != nil {
		t.Fatal(err)
	}

	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)
	configPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, mustConfigName(t, img))

	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case configPath:
			w.Write(mustRawConfigFile(t, img))
		case manifestPath:
			w.Write(mustRawManifest(t, img))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer registryServer.Close()

	u, err := url.Parse(registryServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	ref, err := name.ParseReference(fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo), name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	rmt, err := Image(ref, WithTransport(http.DefaultTransport))
	if err != nil {
		t.Fatal(err)
	}

	layers, err := rmt.Layers()
	if err != nil {
		t.Fatal(err)
	}
	_, err = layers[1].Compressed()
	if err == nil {
		t.Error("Compressed() followed a redirect to a private address")
	}
	if err != nil && !strings.Contains(err.Error(), "private or link-local") {
		t.Logf("Compressed() returned error (expected): %v", err)
	}
}
