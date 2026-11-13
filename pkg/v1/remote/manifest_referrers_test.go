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
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestPutRecoversConcurrentUploadAndUpdatesReferrers(t *testing.T) {
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
			if err := remote.Put(ref, artifact); err != nil {
				t.Fatalf("Put() = %v, want recovery followed by the referrers update", err)
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
