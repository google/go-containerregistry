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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

var fakeDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

func TestGetSchema1(t *testing.T) {
	expectedRepo := "foo/bar"
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Header().Set("Content-Type", string(types.DockerManifestSchema1Signed))
			w.Header().Set("Docker-Content-Digest", fakeDigest)
			w.Write([]byte("doesn't matter"))
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	tag := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo))

	// Get should succeed even for invalid json. We don't parse the response.
	desc, err := Get(tag)
	if err != nil {
		t.Fatalf("Get(%s) = %v", tag, err)
	}

	if desc.Digest.String() != fakeDigest {
		t.Errorf("Descriptor.Digest = %q, expected %q", desc.Digest, fakeDigest)
	}

	want := `unsupported MediaType: "application/vnd.docker.distribution.manifest.v1+prettyjws", see https://github.com/google/go-containerregistry/issues/377`
	// Should fail based on media type.
	if _, err := desc.Image(); err != nil {
		if !errors.Is(err, ErrSchema1) {
			t.Errorf("Image() = %v, expected remote.ErrSchema1", err)
		}
		if diff := cmp.Diff(want, err.Error()); diff != "" {
			t.Errorf("Image() error message (-want +got) = %v", diff)
		}
	} else {
		t.Errorf("Image() = %v, expected err", err)
	}

	// Should fail based on media type.
	if _, err := desc.ImageIndex(); err != nil {
		if !errors.Is(err, ErrSchema1) {
			t.Errorf("ImageImage() = %v, expected remote.ErrSchema1", err)
		}
	} else {
		t.Errorf("ImageIndex() = %v, expected err", err)
	}
}

func TestGetSchema1DigestUsesBodyHash(t *testing.T) {
	expectedRepo := "foo/bar"
	manifest := []byte("schema1 manifest body")
	bodyDigest, _, err := v1.SHA256(strings.NewReader(string(manifest)))
	if err != nil {
		t.Fatalf("SHA256() = %v", err)
	}
	manifestPath := fmt.Sprintf("/v2/%s/manifests/%s", expectedRepo, bodyDigest.String())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Header().Set("Content-Type", string(types.DockerManifestSchema1Signed))
			w.Header().Set("Docker-Content-Digest", fakeDigest)
			w.Write(manifest)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	digest := mustNewDigest(t, fmt.Sprintf("%s/%s@%s", u.Host, expectedRepo, bodyDigest.String()))
	desc, err := Get(digest)
	if err != nil {
		t.Fatalf("Get(%s) = %v", digest, err)
	}

	if desc.Digest != bodyDigest {
		t.Errorf("Descriptor.Digest = %q, want body digest %q", desc.Digest, bodyDigest)
	}
}

func TestGetSchema1DigestRejectsHeaderOverride(t *testing.T) {
	expectedRepo := "foo/bar"
	manifestPath := fmt.Sprintf("/v2/%s/manifests/%s", expectedRepo, fakeDigest)
	manifest := []byte("schema1 manifest body")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Header().Set("Content-Type", string(types.DockerManifestSchema1Signed))
			w.Header().Set("Docker-Content-Digest", fakeDigest)
			w.Write(manifest)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	digest := mustNewDigest(t, fmt.Sprintf("%s/%s@%s", u.Host, expectedRepo, fakeDigest))
	if _, err := Get(digest); err == nil {
		t.Fatalf("Get(%s): expected error, got nil", digest)
	}
}

func TestGetImageAsIndex(t *testing.T) {
	expectedRepo := "foo/bar"
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Header().Set("Content-Type", string(types.DockerManifestSchema2))
			w.Write([]byte("doesn't matter"))
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	tag := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo))

	// Get should succeed even for invalid json. We don't parse the response.
	desc, err := Get(tag)
	if err != nil {
		t.Fatalf("Get(%s) = %v", tag, err)
	}

	// Should fail based on media type.
	if _, err := desc.ImageIndex(); err == nil {
		t.Errorf("ImageIndex() = %v, expected err", err)
	}
}

func TestHeadSchema1(t *testing.T) {
	expectedRepo := "foo/bar"
	mediaType := types.DockerManifestSchema1Signed
	response := []byte("doesn't matter")
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case manifestPath:
			if r.Method != http.MethodHead {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodHead)
			}
			w.Header().Set("Content-Type", string(mediaType))
			w.Header().Set("Content-Length", strconv.Itoa(len(response)))
			w.Header().Set("Docker-Content-Digest", fakeDigest)
			w.Write(response)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	tag := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo))

	// Head should succeed even for invalid json. We don't parse the response.
	desc, err := Head(tag)
	if err != nil {
		t.Fatalf("Head(%s) = %v", tag, err)
	}

	if desc.MediaType != mediaType {
		t.Errorf("Descriptor.MediaType = %q, expected %q", desc.MediaType, mediaType)
	}

	if desc.Digest.String() != fakeDigest {
		t.Errorf("Descriptor.Digest = %q, expected %q", desc.Digest, fakeDigest)
	}

	if desc.Size != int64(len(response)) {
		t.Errorf("Descriptor.Size = %d, expected %d", desc.Size, len(response))
	}
}

// TestHead_MissingHeaders tests that HEAD responses missing necessary headers
// result in errors.
func TestHead_MissingHeaders(t *testing.T) {
	missingType := "missing-type"
	missingLength := "missing-length"
	missingDigest := "missing-digest"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodHead {
			t.Errorf("Method; got %v, want %v", r.Method, http.MethodHead)
		}
		if !strings.Contains(r.URL.Path, missingType) {
			w.Header().Set("Content-Type", "My-Media-Type")
		}
		if !strings.Contains(r.URL.Path, missingLength) {
			w.Header().Set("Content-Length", "10")
		}
		if !strings.Contains(r.URL.Path, missingDigest) {
			w.Header().Set("Docker-Content-Digest", fakeDigest)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	for _, repo := range []string{missingType, missingLength, missingDigest} {
		tag := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, repo))
		if _, err := Head(tag); err == nil {
			t.Errorf("Head(%q): expected error, got nil", tag)
		}
	}
}

// TestRedactFetchBlob tests that a request to fetchBlob that gets redirected
// to a URL that contains sensitive information has that information redacted
// if the subsequent request fails.
func TestRedactFetchBlob(t *testing.T) {
	ctx := context.Background()
	f := fetcher{
		target: mustNewTag(t, "original.com/repo:latest").Context(),
		client: &http.Client{
			Transport: errTransport{},
		},
	}
	h, err := v1.NewHash(fakeDigest)
	if err != nil {
		t.Fatal("NewHash:", err)
	}
	if _, err := f.fetchBlob(ctx, 0, h); err == nil {
		t.Fatalf("fetchBlob: expected error, got nil")
	} else if !strings.Contains(err.Error(), "access_token=REDACTED") {
		t.Fatalf("fetchBlob: expected error to contain redacted access token, got %v", err)
	}
}

type errTransport struct{}

func (errTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// This simulates a registry that returns a redirect upon the first
	// request, and then returns an error upon subsequent requests. This helps
	// test whether error redaction takes into account URLs in error messasges
	// that are not the original request URL.
	if req.URL.Host == "original.com" {
		return &http.Response{
			StatusCode: http.StatusSeeOther,
			Header:     http.Header{"Location": []string{"https://redirected.com?access_token=SECRET"}},
		}, nil
	}
	return nil, fmt.Errorf("error reaching %s", req.URL.String())
}

// TestGet_ArtifactTypeAndAnnotations tests that Get returns the correct
// artifactType and annotations on the descriptor, following the OCI
// distribution spec:
//   - If artifactType is set in the manifest, use it.
//   - Otherwise, fall back to config.mediaType.
//   - Annotations from the manifest are copied to the descriptor.
func TestGet_ArtifactTypeAndAnnotations(t *testing.T) {
	// We need a valid digest in config descriptors so that
	// v1.ParseManifest can unmarshal the JSON without error.
	cfgDigest, err := v1.NewHash(fakeDigest)
	if err != nil {
		t.Fatalf("v1.NewHash: %v", err)
	}

	for _, tc := range []struct {
		desc             string
		manifest         v1.Manifest
		wantArtifactType string
		wantAnnotations  map[string]string
	}{{
		desc: "no artifactType, standard config mediaType falls back to config.mediaType",
		manifest: v1.Manifest{
			SchemaVersion: 2,
			MediaType:     types.OCIManifestSchema1,
			Config: v1.Descriptor{
				MediaType: types.OCIConfigJSON,
				Digest:    cfgDigest,
				Size:      1,
			},
		},
		wantArtifactType: string(types.OCIConfigJSON),
	}, {
		desc: "no artifactType, custom config mediaType falls back to config.mediaType",
		manifest: v1.Manifest{
			SchemaVersion: 2,
			MediaType:     types.OCIManifestSchema1,
			Config: v1.Descriptor{
				MediaType: "application/vnd.custom.thing",
				Digest:    cfgDigest,
				Size:      1,
			},
		},
		wantArtifactType: "application/vnd.custom.thing",
	}, {
		desc: "explicit artifactType takes precedence over config.mediaType",
		manifest: v1.Manifest{
			SchemaVersion: 2,
			MediaType:     types.OCIManifestSchema1,
			Config: v1.Descriptor{
				MediaType: types.OCIConfigJSON,
				Digest:    cfgDigest,
				Size:      1,
			},
			ArtifactType: "application/vnd.my.artifact",
		},
		wantArtifactType: "application/vnd.my.artifact",
	}, {
		desc: "annotations are copied to the descriptor",
		manifest: v1.Manifest{
			SchemaVersion: 2,
			MediaType:     types.OCIManifestSchema1,
			Config: v1.Descriptor{
				MediaType: types.OCIConfigJSON,
				Digest:    cfgDigest,
				Size:      1,
			},
			Annotations: map[string]string{
				"org.opencontainers.image.created": "2024-01-01T00:00:00Z",
				"org.opencontainers.image.authors": "test",
			},
		},
		wantArtifactType: string(types.OCIConfigJSON),
		wantAnnotations: map[string]string{
			"org.opencontainers.image.created": "2024-01-01T00:00:00Z",
			"org.opencontainers.image.authors": "test",
		},
	}, {
		desc: "artifactType and annotations together",
		manifest: v1.Manifest{
			SchemaVersion: 2,
			MediaType:     types.OCIManifestSchema1,
			Config: v1.Descriptor{
				MediaType: types.OCIConfigJSON,
				Digest:    cfgDigest,
				Size:      1,
			},
			ArtifactType: "application/vnd.my.artifact",
			Annotations: map[string]string{
				"foo": "bar",
			},
		},
		wantArtifactType: "application/vnd.my.artifact",
		wantAnnotations: map[string]string{
			"foo": "bar",
		},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			manifestBytes, err := json.Marshal(tc.manifest)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}

			expectedRepo := "foo/bar"
			manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/":
					w.WriteHeader(http.StatusOK)
				case manifestPath:
					w.Header().Set("Content-Type", string(types.OCIManifestSchema1))
					w.Write(manifestBytes)
				default:
					t.Fatalf("Unexpected path: %v", r.URL.Path)
				}
			}))
			defer server.Close()
			u, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("url.Parse(%v) = %v", server.URL, err)
			}

			tag := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo))
			desc, err := Get(tag)
			if err != nil {
				t.Fatalf("Get(%s) = %v", tag, err)
			}

			if got := desc.ArtifactType; got != tc.wantArtifactType {
				t.Errorf("ArtifactType: got %q, want %q", got, tc.wantArtifactType)
			}

			if diff := cmp.Diff(tc.wantAnnotations, desc.Annotations); diff != "" {
				t.Errorf("Annotations (-want +got):\n%s", diff)
			}
		})
	}
}

// TestGet_NonManifestMediaType tests that non-parseable manifests don't
// produce an artifactType or annotations (the parse failure is silently ignored).
func TestGet_NonManifestMediaType(t *testing.T) {
	expectedRepo := "foo/bar"
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case manifestPath:
			w.Header().Set("Content-Type", string(types.DockerManifestSchema1Signed))
			w.Header().Set("Docker-Content-Digest", fakeDigest)
			w.Write([]byte("not valid json"))
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	tag := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo))
	desc, err := Get(tag)
	if err != nil {
		t.Fatalf("Get(%s) = %v", tag, err)
	}

	if got := desc.ArtifactType; got != "" {
		t.Errorf("ArtifactType: got %q, want empty", got)
	}
	if desc.Annotations != nil {
		t.Errorf("Annotations: got %v, want nil", desc.Annotations)
	}
}
