// Copyright 2021 Google LLC All Rights Reserved.
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

package crane

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestDigest_MissingDigest(t *testing.T) {
	response := []byte("doesn't matter")
	digest := "sha256:477c34d98f9e090a4441cf82d2f1f03e64c8eb730e8c1ef39a8595e685d4df65" // Digest of "doesn't matter"
	getCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", string(types.DockerManifestSchema2))
		if r.Method == http.MethodGet {
			getCalled = true
			w.Header().Set("Docker-Content-Digest", digest)
		}
		// This will automatically set the Content-Length header.
		w.Write(response)
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	got, err := Digest(fmt.Sprintf("%s/repo:latest", u.Host))
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if got != digest {
		t.Errorf("Digest: got %q, want %q", got, digest)
	}
	if !getCalled {
		t.Errorf("Digest: expected GET to be called")
	}
}
