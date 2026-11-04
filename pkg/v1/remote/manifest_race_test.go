// Copyright 2026 The go-containerregistry Authors.
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

package remote_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

func mustNewTag(t *testing.T, s string) name.Tag {
	t.Helper()
	tag, err := name.NewTag(s, name.WeakValidation)
	if err != nil {
		t.Fatal(err)
	}
	return tag
}

func TestPushIndexAfterConcurrentManifestUpload(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			child := empty.Image
			childDigest, err := child.Digest()
			if err != nil {
				t.Fatal(err)
			}
			idx := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{Add: child})
			indexDigest, err := idx.Digest()
			if err != nil {
				t.Fatal(err)
			}

			backend := registry.New()
			childPath := "/v2/repo/manifests/" + childDigest.String()
			var initialChecks, rejectedPuts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == childPath {
					if r.Method == http.MethodHead && rejectedPuts.Load() == 0 {
						initialChecks.Add(1)
					}
					if r.Method == http.MethodPut {
						// A competing writer publishes after our HEAD, before our PUT fails.
						result := httptest.NewRecorder()
						backend.ServeHTTP(result, r)
						if result.Code != http.StatusCreated {
							t.Errorf("competing upload status = %d, want %d: %s", result.Code, http.StatusCreated, result.Body)
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						rejectedPuts.Add(1)
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(status)
						if _, err := fmt.Fprint(w, `{"errors":[{"code":"UNAUTHORIZED","message":"No permission to overwrite manifest"}]}`); err != nil {
							t.Errorf("write response: %v", err)
						}
						return
					}
				}
				backend.ServeHTTP(w, r)
			}))
			defer server.Close()
			tag := mustNewTag(t, strings.TrimPrefix(server.URL, "http://")+"/repo:latest")

			if err := remote.Push(tag, idx); err != nil {
				t.Fatalf("Push() = %v, want success after the concurrent upload", err)
			}
			if got := initialChecks.Load(); got != 1 {
				t.Errorf("initial child HEAD count = %d, want 1", got)
			}
			if got := rejectedPuts.Load(); got != 1 {
				t.Errorf("rejected child PUT count = %d, want 1", got)
			}
			for _, ref := range []name.Reference{tag, tag.Context().Digest(indexDigest.String())} {
				desc, err := remote.Head(ref)
				if err != nil {
					t.Fatalf("Head(%s) = %v, want the published index", ref, err)
				}
				if desc.Digest != indexDigest {
					t.Errorf("Head(%s).Digest = %s, want %s", ref, desc.Digest, indexDigest)
				}
			}
		})
	}
}

func TestPutPreservesUploadError(t *testing.T) {
	img := empty.Image
	digest, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := img.RawManifest()
	if err != nil {
		t.Fatal(err)
	}
	mediaType, err := img.MediaType()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		putStatus  int
		headStatus int
		byTag      bool
		mismatch   bool
	}{
		{name: "absent", putStatus: http.StatusUnauthorized, headStatus: http.StatusNotFound},
		{name: "mismatched digest", putStatus: http.StatusForbidden, headStatus: http.StatusOK, mismatch: true},
		{name: "unreadable", putStatus: http.StatusUnauthorized, headStatus: http.StatusForbidden},
		{name: "HEAD fails", putStatus: http.StatusForbidden, headStatus: http.StatusInternalServerError},
		{name: "tag unauthorized", putStatus: http.StatusUnauthorized, headStatus: http.StatusOK, byTag: true},
		{name: "tag forbidden", putStatus: http.StatusForbidden, headStatus: http.StatusOK, byTag: true},
		{name: "other PUT error", putStatus: http.StatusBadRequest, headStatus: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v2/" {
					w.WriteHeader(http.StatusOK)
					return
				}
				switch r.Method {
				case http.MethodPut:
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(test.putStatus)
					if _, err := fmt.Fprint(w, `{"errors":[{"code":"UNAUTHORIZED","message":"original upload denied"}]}`); err != nil {
						t.Errorf("write response: %v", err)
					}
				case http.MethodHead:
					w.Header().Set("Content-Type", string(mediaType))
					w.Header().Set("Content-Length", fmt.Sprint(len(raw)))
					gotDigest := digest.String()
					if test.mismatch {
						gotDigest = "sha256:" + strings.Repeat("0", 64)
					}
					w.Header().Set("Docker-Content-Digest", gotDigest)
					w.WriteHeader(test.headStatus)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()
			tag := mustNewTag(t, strings.TrimPrefix(server.URL, "http://")+"/repo:latest")
			var ref name.Reference = tag.Context().Digest(digest.String())
			if test.byTag {
				ref = tag
			}
			err := remote.Put(ref, img,
				remote.WithRetryPredicate(func(error) bool { return false }),
				remote.WithRetryStatusCodes(),
			)
			var got *transport.Error
			if !errors.As(err, &got) {
				t.Fatalf("Put() = %v, want the original upload error", err)
			}
			if got.Request == nil {
				t.Fatal("upload error has no request")
			}
			if got.StatusCode != test.putStatus || got.Request.Method != http.MethodPut {
				t.Errorf("Put() = %v, want the original PUT status %d", got, test.putStatus)
			}
			if len(got.Errors) != 1 || got.Errors[0].Message != "original upload denied" {
				t.Errorf("Put() diagnostics = %v, want the original upload denial", got.Errors)
			}
		})
	}
}
