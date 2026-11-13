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
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestPutPreservesReferrersError(t *testing.T) {
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
	referrersPath := "/v2/repo/referrers/" + subject.String()
	for _, test := range []struct {
		name      string
		byTag     bool
		putStatus int
	}{
		{name: "digest", putStatus: http.StatusCreated},
		{name: "tag", byTag: true, putStatus: http.StatusCreated},
		{name: "recovered tag", byTag: true, putStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			identifier := digest.String()
			if test.byTag {
				identifier = "latest"
			}
			manifestPath := "/v2/repo/manifests/" + identifier
			var puts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/v2/":
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
			err := remote.Put(ref, img)
			var got *transport.Error
			if !errors.As(err, &got) {
				t.Fatalf("Put() = %v, want the original referrers error", err)
			}
			if got.StatusCode != http.StatusForbidden || got.Request == nil || got.Request.Method != http.MethodGet || got.Request.URL.Path != referrersPath {
				t.Errorf("Put() = %v, want the original referrers GET status 403", got)
			}
			if len(got.Errors) != 1 || got.Errors[0].Message != "referrers read denied" {
				t.Errorf("Put() diagnostics = %v, want the original referrers denial", got.Errors)
			}
			if got := puts.Load(); got != 1 {
				t.Errorf("primary manifest PUT count = %d, want 1", got)
			}
		})
	}
}
