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

func TestGetSchema1(t *testing.T) {
	expectedRepo := "foo/bar"
	fakeDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"
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
		if errors.Is(err, &ErrSchema1{}) {
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
		var s1err ErrSchema1
		if errors.Is(err, &s1err) {
			t.Errorf("ImageImage() = %v, expected remote.ErrSchema1", err)
		}
	} else {
		t.Errorf("ImageIndex() = %v, expected err", err)
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
	fakeDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"
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
		t.Errorf("Descriptor.Size = %q, expected %q", desc.Size, len(response))
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
			w.Header().Set("Docker-Content-Digest", "sha256:0000000000000000000000000000000000000000000000000000000000000000")
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
		Ref: mustNewTag(t, "original.com/repo:latest"),
		Client: &http.Client{
			Transport: errTransport{},
		},
		context: ctx,
	}
	h, err := v1.NewHash("sha256:0000000000000000000000000000000000000000000000000000000000000000")
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
