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

package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
)

func mustParseDuration(t *testing.T, d string) time.Duration {
	dur, err := time.ParseDuration(d)
	if err != nil {
		t.Fatal(err)
	}
	return dur
}

func TestRoundtrip(t *testing.T) {
	raw := rawManifestInfo{
		Size:      "100",
		MediaType: "hi",
		Created:   "12345678",
		Uploaded:  "23456789",
		Tags:      []string{"latest"},
	}

	og, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	parsed := ManifestInfo{}
	if err := json.Unmarshal(og, &parsed); err != nil {
		t.Fatal(err)
	}

	roundtripped, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(og, roundtripped); diff != "" {
		t.Errorf("ManifestInfo can't roundtrip: (-want +got) = %s", diff)
	}
}

func TestList(t *testing.T) {
	cases := []struct {
		name         string
		responseBody []byte
		wantErr      bool
		wantTags     *Tags
	}{{
		name:         "success",
		responseBody: []byte(`{"tags":["foo","bar"]}`),
		wantErr:      false,
		wantTags:     &Tags{Tags: []string{"foo", "bar"}},
	}, {
		name:         "gcr success",
		responseBody: []byte(`{"child":["hello", "world"],"manifest":{"digest1":{"imageSizeBytes":"1","mediaType":"mainstream","timeCreatedms":"1","timeUploadedMs":"2","tag":["foo"]},"digest2":{"imageSizeBytes":"2","mediaType":"indie","timeCreatedMs":"3","timeUploadedMs":"4","tag":["bar","baz"]}},"tags":["foo","bar","baz"]}`),
		wantErr:      false,
		wantTags: &Tags{
			Children: []string{"hello", "world"},
			Manifests: map[string]ManifestInfo{
				"digest1": {
					Size:      1,
					MediaType: "mainstream",
					Created:   time.Unix(0, 0).Add(mustParseDuration(t, "1ms")),
					Uploaded:  time.Unix(0, 0).Add(mustParseDuration(t, "2ms")),
					Tags:      []string{"foo"},
				},
				"digest2": {
					Size:      2,
					MediaType: "indie",
					Created:   time.Unix(0, 0).Add(mustParseDuration(t, "3ms")),
					Uploaded:  time.Unix(0, 0).Add(mustParseDuration(t, "4ms")),
					Tags:      []string{"bar", "baz"},
				},
			},
			Tags: []string{"foo", "bar", "baz"},
		},
	}, {
		name:         "just children",
		responseBody: []byte(`{"child":["hello", "world"]}`),
		wantErr:      false,
		wantTags: &Tags{
			Children: []string{"hello", "world"},
		},
	}, {
		name:         "not json",
		responseBody: []byte("notjson"),
		wantErr:      true,
	}}

	repoName := "ubuntu"
	// To test WithUserAgent
	uaSentinel := "this-is-the-user-agent"

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tagsPath := fmt.Sprintf("/v2/%s/tags/list", repoName)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got, want := r.Header.Get("User-Agent"), uaSentinel; !strings.Contains(got, want) {
					t.Errorf("request did not container useragent, got %q want Contains(%q)", got, want)
				}
				switch r.URL.Path {
				case "/v2/":
					w.WriteHeader(http.StatusOK)
				case tagsPath:
					if r.Method != http.MethodGet {
						t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
					}

					w.Write(tc.responseBody)
				default:
					t.Fatalf("Unexpected path: %v", r.URL.Path)
				}
			}))
			defer server.Close()
			u, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("url.Parse(%v) = %v", server.URL, err)
			}

			repo, err := name.NewRepository(fmt.Sprintf("%s/%s", u.Host, repoName), name.WeakValidation)
			if err != nil {
				t.Fatalf("name.NewRepository(%v) = %v", repoName, err)
			}

			tags, err := List(repo, WithAuthFromKeychain(authn.DefaultKeychain), WithTransport(http.DefaultTransport), WithUserAgent(uaSentinel), WithContext(context.Background()))
			if (err != nil) != tc.wantErr {
				t.Errorf("List() wrong error: %v, want %v: %v\n", (err != nil), tc.wantErr, err)
			}

			if diff := cmp.Diff(tc.wantTags, tags); diff != "" {
				t.Errorf("List() wrong tags (-want +got) = %s", diff)
			}
		})
	}
}

type recorder struct {
	Tags []*Tags
	Errs []error
}

func (r *recorder) walk(_ name.Repository, tags *Tags, err error) error {
	r.Tags = append(r.Tags, tags)
	r.Errs = append(r.Errs, err)

	return nil
}

func TestWalk(t *testing.T) {
	// Stupid coverage to make sure it doesn't panic.
	var b bytes.Buffer
	logs.Debug.SetOutput(&b)

	cases := []struct {
		name         string
		responseBody []byte
		wantResult   recorder
	}{{
		name:         "gcr success",
		responseBody: []byte(`{"child":["hello", "world"],"manifest":{"digest1":{"imageSizeBytes":"1","mediaType":"mainstream","timeCreatedms":"1","timeUploadedMs":"2","tag":["foo"]},"digest2":{"imageSizeBytes":"2","mediaType":"indie","timeCreatedMs":"3","timeUploadedMs":"4","tag":["bar","baz"]}},"tags":["foo","bar","baz"]}`),
		wantResult: recorder{
			Tags: []*Tags{{
				Children: []string{"hello", "world"},
				Manifests: map[string]ManifestInfo{
					"digest1": {
						Size:      1,
						MediaType: "mainstream",
						Created:   time.Unix(0, 0).Add(mustParseDuration(t, "1ms")),
						Uploaded:  time.Unix(0, 0).Add(mustParseDuration(t, "2ms")),
						Tags:      []string{"foo"},
					},
					"digest2": {
						Size:      2,
						MediaType: "indie",
						Created:   time.Unix(0, 0).Add(mustParseDuration(t, "3ms")),
						Uploaded:  time.Unix(0, 0).Add(mustParseDuration(t, "4ms")),
						Tags:      []string{"bar", "baz"},
					},
				},
				Tags: []string{"foo", "bar", "baz"},
			}, {
				Tags: []string{"hello"},
			}, {
				Tags: []string{"world"},
			}},
			Errs: []error{nil, nil, nil},
		},
	}}

	repoName := "ubuntu"

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rootPath := fmt.Sprintf("/v2/%s/tags/list", repoName)
			helloPath := fmt.Sprintf("/v2/%s/hello/tags/list", repoName)
			worldPath := fmt.Sprintf("/v2/%s/world/tags/list", repoName)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/":
					w.WriteHeader(http.StatusOK)
				case rootPath:
					if r.Method != http.MethodGet {
						t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
					}

					w.Write(tc.responseBody)
				case helloPath:
					w.Write([]byte(`{"tags":["hello"]}`))
				case worldPath:
					w.Write([]byte(`{"tags":["world"]}`))
				default:
					t.Fatalf("Unexpected path: %v", r.URL.Path)
				}
			}))
			defer server.Close()
			u, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("url.Parse(%v) = %v", server.URL, err)
			}

			repo, err := name.NewRepository(fmt.Sprintf("%s/%s", u.Host, repoName), name.WeakValidation)
			if err != nil {
				t.Fatalf("name.NewRepository(%v) = %v", repoName, err)
			}

			r := recorder{}
			if err := Walk(repo, r.walk, WithAuth(authn.Anonymous)); err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			if diff := cmp.Diff(tc.wantResult, r); diff != "" {
				t.Errorf("Walk() wrong tags (-want +got) = %s", diff)
			}
		})
	}
}

// Copied shamelessly from remote.
func TestCancelledList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repoName := "doesnotmatter"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	repo, err := name.NewRepository(fmt.Sprintf("%s/%s", u.Host, repoName), name.WeakValidation)
	if err != nil {
		t.Fatalf("name.NewRepository(%v) = %v", repoName, err)
	}

	_, err = List(repo, WithContext(ctx))
	if !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Errorf("wanted %q to contain %q", err.Error(), context.Canceled.Error())
	}
}

func makeResp(hdr string) *http.Response {
	return &http.Response{
		Header: http.Header{
			"Link": []string{hdr},
		},
		Request: &http.Request{
			URL: &url.URL{
				Scheme: "https",
				Host:   "example.com",
			},
		},
	}
}

func TestGetNextPageURL(t *testing.T) {
	repo, err := name.NewRepository("example.com/myrepo")
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	for _, hdr := range []string{
		"",
		"<",
		"><",
		"<>",
		fmt.Sprintf("<%c>", 0x7f), // makes url.Parse fail
	} {
		u, err := getNextPageURL(makeResp(hdr), repo)
		if err == nil && u != nil {
			t.Errorf("Expected err or nil URL for %q, got %+v", hdr, u)
		}
	}

	good := &http.Response{
		Header: http.Header{
			"Link": []string{"</v2/myrepo/tags/list?n=100>"},
		},
		Request: &http.Request{
			URL: &url.URL{
				Scheme: "https",
				Host:   "example.com",
				Path:   "/v2/myrepo/tags/list",
			},
		},
	}
	u, err := getNextPageURL(good, repo)
	if err != nil {
		t.Fatal(err)
	}

	if u.Scheme != "https" {
		t.Errorf("expected scheme to match request, got %s", u.Scheme)
	}
	if u.Host != "example.com" {
		t.Errorf("expected host to match request, got %s", u.Host)
	}
}

func TestValidatePaginationURL(t *testing.T) {
	repo, err := name.NewRepository("registry.example.com/myrepo")
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "same host - valid",
			url:     "https://registry.example.com/v2/myrepo/tags/list?n=100&last=tag1",
			wantErr: false,
		},
		{
			name:    "cloud metadata endpoint - SSRF attempt",
			url:     "http://169.254.169.254/latest/meta-data/",
			wantErr: true,
		},
		{
			name:    "localhost - SSRF attempt",
			url:     "http://localhost:8080/internal",
			wantErr: true,
		},
		{
			name:    "internal IP - SSRF attempt",
			url:     "http://192.168.1.1/admin",
			wantErr: true,
		},
		{
			name:    "different registry - SSRF attempt",
			url:     "https://evil-registry.com/v2/myrepo/tags/list",
			wantErr: true,
		},
		{
			name:    "scheme mismatch - potential downgrade",
			url:     "http://registry.example.com/v2/myrepo/tags/list",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.url)
			if err != nil {
				t.Fatalf("failed to parse URL: %v", err)
			}

			err = validatePaginationURL(u, repo)
			if tc.wantErr && err == nil {
				t.Errorf("validatePaginationURL(%q) = nil, want error", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validatePaginationURL(%q) = %v, want nil", tc.url, err)
			}
		})
	}
}

func TestGetNextPageURL_SSRF(t *testing.T) {
	repo, _ := name.NewRepository("registry.example.com/myrepo")

	// Malicious registry returns Link header pointing to cloud metadata
	malicious := &http.Response{
		Header: http.Header{
			"Link": []string{"<http://169.254.169.254/latest/meta-data/>;rel=\"next\""},
		},
		Request: &http.Request{
			URL: &url.URL{
				Scheme: "https",
				Host:   "registry.example.com",
				Path:   "/v2/myrepo/tags/list",
			},
		},
	}

	_, err := getNextPageURL(malicious, repo)
	if err == nil {
		t.Error("getNextPageURL should reject Link header pointing to cloud metadata endpoint")
	}
}
