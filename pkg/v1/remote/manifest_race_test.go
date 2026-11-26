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
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
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
	for _, test := range []struct {
		name   string
		status int
		byTag  bool
	}{
		{name: "digest unauthorized", status: http.StatusUnauthorized},
		{name: "digest forbidden", status: http.StatusForbidden},
		{name: "tag unauthorized", status: http.StatusUnauthorized, byTag: true},
		{name: "tag forbidden", status: http.StatusForbidden, byTag: true},
	} {
		t.Run(test.name, func(t *testing.T) {
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
			manifestPath := "/v2/repo/manifests/" + childDigest.String()
			if test.byTag {
				manifestPath = "/v2/repo/manifests/latest"
			}
			var initialChecks, rejectedPuts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == manifestPath {
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
						w.WriteHeader(test.status)
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
				t.Errorf("initial manifest HEAD count = %d, want 1", got)
			}
			if got := rejectedPuts.Load(); got != 1 {
				t.Errorf("rejected manifest PUT count = %d, want 1", got)
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

func TestManifestUploadAfterDeniedPut(t *testing.T) {
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
	// Exercise manifest writes without uploading blobs.
	taggable := struct{ remote.Taggable }{img}
	tests := []struct {
		name        string
		operation   string
		putStatus   int
		headStatus  int
		byTag       bool
		mismatch    bool
		omitDigest  bool
		wantSuccess bool
	}{
		{name: "absent", putStatus: http.StatusUnauthorized, headStatus: http.StatusNotFound},
		{name: "mismatched digest", putStatus: http.StatusForbidden, headStatus: http.StatusOK, mismatch: true},
		{name: "unreadable", putStatus: http.StatusUnauthorized, headStatus: http.StatusForbidden},
		{name: "HEAD fails", putStatus: http.StatusForbidden, headStatus: http.StatusInternalServerError},
		{name: "tag unauthorized", putStatus: http.StatusUnauthorized, headStatus: http.StatusOK, byTag: true, wantSuccess: true},
		{name: "tag forbidden", putStatus: http.StatusForbidden, headStatus: http.StatusOK, byTag: true, wantSuccess: true},
		{name: "tag absent", putStatus: http.StatusUnauthorized, headStatus: http.StatusNotFound, byTag: true},
		{name: "tag mismatched digest", putStatus: http.StatusForbidden, headStatus: http.StatusOK, byTag: true, mismatch: true},
		{name: "tag unreadable", putStatus: http.StatusUnauthorized, headStatus: http.StatusForbidden, byTag: true},
		{name: "tag HEAD fails", putStatus: http.StatusForbidden, headStatus: http.StatusInternalServerError, byTag: true},
		{name: "tag HEAD missing digest", putStatus: http.StatusForbidden, headStatus: http.StatusOK, byTag: true, omitDigest: true},
		{name: "tag other PUT error", putStatus: http.StatusBadRequest, headStatus: http.StatusOK, byTag: true},
		{name: "other PUT error", putStatus: http.StatusBadRequest, headStatus: http.StatusOK},
		{name: "direct Put by digest", operation: "Put", putStatus: http.StatusForbidden, headStatus: http.StatusOK},
		{name: "direct Put by tag", operation: "Put", putStatus: http.StatusUnauthorized, headStatus: http.StatusOK, byTag: true},
		{name: "direct Tag", operation: "Tag", putStatus: http.StatusForbidden, headStatus: http.StatusOK, byTag: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifestPath := "/v2/repo/manifests/" + digest.String()
			if test.byTag {
				manifestPath = "/v2/repo/manifests/latest"
			}
			var initialHeads, rejectedPuts, recoveryHeads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v2/" {
					w.WriteHeader(http.StatusOK)
					return
				}
				if r.URL.Path != manifestPath {
					t.Errorf("unexpected target: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				switch r.Method {
				case http.MethodPut:
					rejectedPuts.Add(1)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(test.putStatus)
					if _, err := fmt.Fprint(w, `{"errors":[{"code":"UNAUTHORIZED","message":"original upload denied"}]}`); err != nil {
						t.Errorf("write response: %v", err)
					}
				case http.MethodHead:
					if rejectedPuts.Load() == 0 {
						initialHeads.Add(1)
						w.WriteHeader(http.StatusNotFound)
						return
					}
					recoveryHeads.Add(1)
					w.Header().Set("Content-Type", string(mediaType))
					w.Header().Set("Content-Length", fmt.Sprint(len(raw)))
					gotDigest := digest.String()
					if test.mismatch {
						gotDigest = "sha256:" + strings.Repeat("0", 64)
					}
					if !test.omitDigest {
						w.Header().Set("Docker-Content-Digest", gotDigest)
					}
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
			options := []remote.Option{
				remote.WithRetryPredicate(func(error) bool { return false }),
				remote.WithRetryStatusCodes(),
			}
			var err error
			var wantInitialHeads, wantRecoveryHeads int32
			operation := test.operation
			switch operation {
			case "Put":
				err = remote.Put(ref, taggable, options...)
			case "Tag":
				err = remote.Tag(tag, taggable, options...)
			default:
				operation = "Push"
				wantInitialHeads = 1
				if test.putStatus == http.StatusUnauthorized || test.putStatus == http.StatusForbidden {
					wantRecoveryHeads = 1
				}
				err = remote.Push(ref, taggable, options...)
			}
			if got := initialHeads.Load(); got != wantInitialHeads {
				t.Errorf("initial manifest HEAD count = %d, want %d", got, wantInitialHeads)
			}
			if got := rejectedPuts.Load(); got != 1 {
				t.Errorf("rejected manifest PUT count = %d, want 1", got)
			}
			if got := recoveryHeads.Load(); got != wantRecoveryHeads {
				t.Errorf("manifest HEAD count after denied PUT = %d, want %d", got, wantRecoveryHeads)
			}
			if test.wantSuccess {
				if err != nil {
					t.Fatalf("%s() = %v, want success after matching tag HEAD", operation, err)
				}
				return
			}
			var got *transport.Error
			if !errors.As(err, &got) {
				t.Fatalf("%s() = %v, want the original upload error", operation, err)
			}
			if got.Request == nil {
				t.Fatal("upload error has no request")
			}
			if got.StatusCode != test.putStatus || got.Request.Method != http.MethodPut || got.Request.URL.Path != manifestPath {
				t.Errorf("%s() = %v, want the original PUT status %d", operation, got, test.putStatus)
			}
			if len(got.Errors) != 1 || got.Errors[0].Message != "original upload denied" {
				t.Errorf("%s() diagnostics = %v, want the original upload denial", operation, got.Errors)
			}
		})
	}
}

func TestPushRecoversConcurrentUploadAndUpdatesReferrers(t *testing.T) {
	subject, err := partial.Descriptor(empty.Image)
	if err != nil {
		t.Fatal(err)
	}
	artifact := mutate.Subject(mutate.MediaType(empty.Image, types.OCIManifestSchema1), *subject).(v1.Image)
	digest, err := artifact.Digest()
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name  string
		byTag bool
	}{
		{name: "digest"},
		{name: "tag", byTag: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			identifier := digest.String()
			if test.byTag {
				identifier = "artifact"
			}
			backend := registry.New(registry.WithReferrersSupport(false))
			artifactPath := "/v2/repo/manifests/" + identifier
			var rejectedPuts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPut && r.URL.Path == artifactPath {
					// The competing writer publishes the manifest without updating the fallback tag.
					result := httptest.NewRecorder()
					backend.ServeHTTP(result, r)
					if result.Code != http.StatusCreated {
						t.Errorf("competing upload status = %d, want %d: %s", result.Code, http.StatusCreated, result.Body)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					rejectedPuts.Add(1)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					if _, err := fmt.Fprint(w, `{"errors":[{"code":"UNAUTHORIZED","message":"No permission to overwrite manifest"}]}`); err != nil {
						t.Errorf("write response: %v", err)
					}
					return
				}
				backend.ServeHTTP(w, r)
			}))
			defer server.Close()
			root := mustNewTag(t, strings.TrimPrefix(server.URL, "http://")+"/repo:root")
			if err := remote.Write(root, empty.Image); err != nil {
				t.Fatal(err)
			}
			var ref name.Reference = root.Context().Digest(digest.String())
			if test.byTag {
				ref = root.Context().Tag(identifier)
			}
			if err := remote.Push(ref, artifact); err != nil {
				t.Fatalf("Push() = %v, want recovery followed by the referrers update", err)
			}
			if got := rejectedPuts.Load(); got != 1 {
				t.Errorf("rejected PUT count = %d, want 1", got)
			}

			fallback := root.Context().Tag("sha256-" + subject.Digest.Hex)
			idx, err := remote.Index(fallback)
			if err != nil {
				t.Fatalf("Index(%s) = %v, want the referrers fallback index", fallback, err)
			}
			manifest, err := idx.IndexManifest()
			if err != nil {
				t.Fatal(err)
			}
			if len(manifest.Manifests) != 1 || manifest.Manifests[0].Digest != digest {
				t.Errorf("referrers = %v, want one descriptor for %s", manifest.Manifests, digest)
			}
		})
	}
}

func TestPushPreservesReferrersError(t *testing.T) {
	subject := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("0", 64)}
	img := mutate.Subject(empty.Image, v1.Descriptor{Digest: subject}).(v1.Image)
	digest, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := img.RawManifest()
	if err != nil {
		t.Fatal(err)
	}
	config, err := img.ConfigName()
	if err != nil {
		t.Fatal(err)
	}
	configPath := "/v2/repo/blobs/" + config.String()
	referrersPath := "/v2/repo/referrers/" + subject.String()
	for _, test := range []struct {
		name      string
		byTag     bool
		putStatus int
	}{
		{name: "digest", putStatus: http.StatusCreated},
		{name: "tag", byTag: true, putStatus: http.StatusCreated},
		{name: "recovered digest", putStatus: http.StatusForbidden},
		{name: "recovered tag", byTag: true, putStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			identifier := digest.String()
			if test.byTag {
				identifier = "latest"
			}
			manifestPath := "/v2/repo/manifests/" + identifier
			var puts, recoveryHeads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/v2/":
					w.WriteHeader(http.StatusOK)
				case r.Method == http.MethodHead && r.URL.Path == configPath:
					w.WriteHeader(http.StatusOK)
				case r.Method == http.MethodPut && r.URL.Path == manifestPath:
					puts.Add(1)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(test.putStatus)
					if test.putStatus == http.StatusForbidden {
						if _, err := fmt.Fprint(w, `{"errors":[{"code":"UNAUTHORIZED","message":"No permission to overwrite manifest"}]}`); err != nil {
							t.Errorf("write response: %v", err)
						}
					}
				case r.Method == http.MethodGet && r.URL.Path == referrersPath:
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					if _, err := fmt.Fprint(w, `{"errors":[{"code":"DENIED","message":"referrers read denied"}]}`); err != nil {
						t.Errorf("write response: %v", err)
					}
				case r.Method == http.MethodHead && r.URL.Path == manifestPath:
					if puts.Load() == 0 {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					recoveryHeads.Add(1)
					w.Header().Set("Content-Type", string(types.DockerManifestSchema2))
					w.Header().Set("Content-Length", fmt.Sprint(len(raw)))
					w.Header().Set("Docker-Content-Digest", digest.String())
					w.WriteHeader(http.StatusOK)
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
			err := remote.Push(ref, img)
			var got *transport.Error
			if !errors.As(err, &got) {
				t.Fatalf("Push() = %v, want the original referrers error", err)
			}
			if got.StatusCode != http.StatusForbidden || got.Request == nil || got.Request.Method != http.MethodGet || got.Request.URL.Path != referrersPath {
				t.Errorf("Push() = %v, want the original referrers GET status 403", got)
			}
			if len(got.Errors) != 1 || got.Errors[0].Message != "referrers read denied" {
				t.Errorf("Push() diagnostics = %v, want the original referrers denial", got.Errors)
			}
			if got := puts.Load(); got != 1 {
				t.Errorf("primary manifest PUT count = %d, want 1", got)
			}
			wantRecoveryHeads := int32(0)
			if test.putStatus == http.StatusForbidden {
				wantRecoveryHeads = 1
			}
			if got := recoveryHeads.Load(); got != wantRecoveryHeads {
				t.Errorf("manifest HEAD count after PUT = %d, want %d", got, wantRecoveryHeads)
			}
		})
	}
}
