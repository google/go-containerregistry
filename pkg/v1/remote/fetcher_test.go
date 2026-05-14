// Copyright 2024 Google LLC All Rights Reserved.
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
	"net/http"
	"net/url"
	"testing"
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
