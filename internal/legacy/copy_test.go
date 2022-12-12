// Copyright 2019 Google LLC All Rights Reserved.
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

package legacy

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestCopySchema1(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// We'll copy from src to dst.
	src := fmt.Sprintf("%s/schema1/src", u.Host)
	srcRef, err := name.ParseReference(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := fmt.Sprintf("%s/schema1/dst", u.Host)
	dstRef, err := name.ParseReference(dst)
	if err != nil {
		t.Fatal(err)
	}

	// Create a random layer.
	layer, err := random.Layer(1024, types.DockerLayer)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := layer.Digest()
	if err != nil {
		t.Fatal(err)
	}
	layerRef, err := name.NewDigest(fmt.Sprintf("%s@%s", src, digest))
	if err != nil {
		t.Fatal(err)
	}

	// Populate the registry with a layer and a schema 1 manifest referencing it.
	if err := remote.WriteLayer(layerRef.Context(), layer); err != nil {
		t.Fatal(err)
	}
	manifest := schema1{
		FSLayers: []fslayer{{
			BlobSum: digest.String(),
		}},
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	desc := &remote.Descriptor{
		Manifest: b,
		Descriptor: v1.Descriptor{
			MediaType: types.DockerManifestSchema1,
			Digest: v1.Hash{Algorithm: "sha256",
				Hex: strings.Repeat("a", 64),
			},
		},
	}
	if err := remote.Put(dstRef, desc); err != nil {
		t.Fatal(err)
	}

	if err := CopySchema1(desc, srcRef, dstRef); err != nil {
		t.Errorf("failed to copy schema 1: %v", err)
	}
}
