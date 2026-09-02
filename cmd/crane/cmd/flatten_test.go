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

package cmd

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestFlattenImagePreservesMediaType(t *testing.T) {
	for _, mt := range []types.MediaType{
		types.OCIManifestSchema1,
		types.DockerManifestSchema2,
	} {
		t.Run(string(mt), func(t *testing.T) {
			s := httptest.NewServer(registry.New())
			defer s.Close()
			u, err := url.Parse(s.URL)
			if err != nil {
				t.Fatal(err)
			}

			repo, err := name.NewRepository(fmt.Sprintf("%s/test/flatten", u.Host))
			if err != nil {
				t.Fatal(err)
			}

			wantConfigType := types.DockerConfigJSON
			wantLayerType := types.DockerLayer
			if mt == types.OCIManifestSchema1 {
				wantConfigType = types.OCIConfigJSON
				wantLayerType = types.OCILayer
			}

			img, err := random.Image(1024, 2)
			if err != nil {
				t.Fatalf("random.Image: %v", err)
			}
			img = mutate.MediaType(img, mt)
			img = mutate.ConfigMediaType(img, wantConfigType)

			flat, err := flattenImage(img, repo, "crane", crane.GetOptions())
			if err != nil {
				t.Fatalf("flattenImage: %v", err)
			}

			flatImg, ok := flat.(v1.Image)
			if !ok {
				t.Fatalf("flattenImage returned %T, want v1.Image", flat)
			}

			got, err := flatImg.MediaType()
			if err != nil {
				t.Fatalf("MediaType: %v", err)
			}
			if got != mt {
				t.Errorf("flattened media type = %q, want %q", got, mt)
			}

			m, err := flatImg.Manifest()
			if err != nil {
				t.Fatalf("Manifest: %v", err)
			}
			if m.Config.MediaType != wantConfigType {
				t.Errorf("config media type = %q, want %q", m.Config.MediaType, wantConfigType)
			}
			if len(m.Layers) != 1 {
				t.Fatalf("got %d layers, want 1", len(m.Layers))
			}
			if m.Layers[0].MediaType != wantLayerType {
				t.Errorf("layer media type = %q, want %q", m.Layers[0].MediaType, wantLayerType)
			}
		})
	}
}

func TestFlattenIndexPreservesMediaType(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	repo, err := name.NewRepository(fmt.Sprintf("%s/test/flatten-index", u.Host))
	if err != nil {
		t.Fatal(err)
	}

	img, err := random.Image(1024, 2)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	img = mutate.MediaType(img, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, types.OCIConfigJSON)

	idx := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
		Add: img,
		Descriptor: v1.Descriptor{
			Platform: &v1.Platform{OS: "linux", Architecture: "amd64"},
		},
	})
	idx = mutate.IndexMediaType(idx, types.OCIImageIndex)

	flat, err := flattenIndex(idx, repo, "crane", crane.GetOptions())
	if err != nil {
		t.Fatalf("flattenIndex: %v", err)
	}

	flatIdx, ok := flat.(v1.ImageIndex)
	if !ok {
		t.Fatalf("flattenIndex returned %T, want v1.ImageIndex", flat)
	}

	gotMT, err := flatIdx.MediaType()
	if err != nil {
		t.Fatalf("MediaType: %v", err)
	}
	if gotMT != types.OCIImageIndex {
		t.Errorf("flattened index media type = %q, want %q", gotMT, types.OCIImageIndex)
	}

	im, err := flatIdx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest: %v", err)
	}
	if len(im.Manifests) != 1 {
		t.Fatalf("got %d manifests, want 1", len(im.Manifests))
	}
	desc := im.Manifests[0]
	if desc.MediaType != types.OCIManifestSchema1 {
		t.Errorf("child descriptor media type = %q, want %q", desc.MediaType, types.OCIManifestSchema1)
	}
	if desc.ArtifactType == string(types.DockerConfigJSON) {
		t.Errorf("child descriptor artifactType should not be Docker config JSON")
	}

	child, err := flatIdx.Image(desc.Digest)
	if err != nil {
		t.Fatalf("getting child image: %v", err)
	}
	childMf, err := child.Manifest()
	if err != nil {
		t.Fatalf("child Manifest: %v", err)
	}
	if childMf.Config.MediaType != types.OCIConfigJSON {
		t.Errorf("child config media type = %q, want %q", childMf.Config.MediaType, types.OCIConfigJSON)
	}
	if len(childMf.Layers) != 1 || childMf.Layers[0].MediaType != types.OCILayer {
		t.Errorf("child layers = %v, want 1 layer of type %q", childMf.Layers, types.OCILayer)
	}
}
