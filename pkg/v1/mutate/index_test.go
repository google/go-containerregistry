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

package mutate_test

import (
	"log"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestAppendIndex(t *testing.T) {
	base, err := random.Index(1024, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := random.Index(2048, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	img, err := random.Image(4096, 5)
	if err != nil {
		t.Fatal(err)
	}
	l, err := random.Layer(1024, types.OCIUncompressedRestrictedLayer)
	if err != nil {
		t.Fatal(err)
	}

	weirdHash := v1.Hash{
		Algorithm: "sha256",
		Hex:       strings.Repeat("0", 64),
	}

	add := mutate.AppendManifests(base, mutate.IndexAddendum{
		Add: idx,
		Descriptor: v1.Descriptor{
			URLs: []string{"index.example.com"},
		},
	}, mutate.IndexAddendum{
		Add: img,
		Descriptor: v1.Descriptor{
			URLs: []string{"image.example.com"},
		},
	}, mutate.IndexAddendum{
		Add: l,
		Descriptor: v1.Descriptor{
			MediaType: types.MediaType("application/xml"),
			URLs:      []string{"blob.example.com"},
		},
	}, mutate.IndexAddendum{
		Add: l,
		Descriptor: v1.Descriptor{
			URLs:   []string{"layer.example.com"},
			Size:   1337,
			Digest: weirdHash,
			Platform: &v1.Platform{
				OS:           "haiku",
				Architecture: "toaster",
			},
			Annotations: map[string]string{"weird": "true"},
		},
	})

	if err := validate.Index(add); err != nil {
		t.Errorf("Validate() = %v", err)
	}

	got, err := add.MediaType()
	if err != nil {
		t.Fatal(err)
	}
	want, err := base.MediaType()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("MediaType() = %s != %s", got, want)
	}

	// TODO(jonjohnsonjr): There's no way to grab layers from v1.ImageIndex.
	m, err := add.IndexManifest()
	if err != nil {
		log.Fatal(err)
	}

	for i, want := range map[int]string{
		3: "index.example.com",
		4: "image.example.com",
		5: "blob.example.com",
		6: "layer.example.com",
	} {
		if got := m.Manifests[i].URLs[0]; got != want {
			t.Errorf("wrong URLs[0] for Manifests[%d]: %s != %s", i, got, want)
		}
	}

	if got, want := m.Manifests[5].MediaType, types.MediaType("application/xml"); got != want {
		t.Errorf("wrong MediaType for layer: %s != %s", got, want)
	}

	if got, want := m.Manifests[6].MediaType, types.OCIUncompressedRestrictedLayer; got != want {
		t.Errorf("wrong MediaType for layer: %s != %s", got, want)
	}

	// Append the index to itself and make sure it still validates.
	add = mutate.AppendManifests(add, mutate.IndexAddendum{
		Add: add,
	})
	if err := validate.Index(add); err != nil {
		t.Errorf("Validate() = %v", err)
	}

	// Wrap the whole thing in another index and make sure it still validates.
	add = mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
		Add: add,
	})
	if err := validate.Index(add); err != nil {
		t.Errorf("Validate() = %v", err)
	}
}

func TestIndexImmutability(t *testing.T) {
	base, err := random.Index(1024, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	ii, err := random.Index(2048, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	i, err := random.Image(4096, 5)
	if err != nil {
		t.Fatal(err)
	}
	idx := mutate.AppendManifests(base, mutate.IndexAddendum{
		Add: ii,
		Descriptor: v1.Descriptor{
			URLs: []string{"index.example.com"},
		},
	}, mutate.IndexAddendum{
		Add: i,
		Descriptor: v1.Descriptor{
			URLs: []string{"image.example.com"},
		},
	})

	t.Run("index manifest", func(t *testing.T) {
		// Check that Manifest is immutable.
		changed, err := idx.IndexManifest()
		if err != nil {
			t.Errorf("IndexManifest() = %v", err)
		}
		want := changed.DeepCopy() // Create a copy of original before mutating it.
		changed.MediaType = types.DockerManifestList

		if got, err := idx.IndexManifest(); err != nil {
			t.Errorf("IndexManifest() = %v", err)
		} else if !cmp.Equal(got, want) {
			t.Errorf("IndexManifest changed! %s", cmp.Diff(got, want))
		}
	})
}

// TestAppend_ArtifactType tests that appending an image manifest that has a
// non-standard config.mediaType to an index, results in the image's
// config.mediaType being hoisted into the descriptor inside the index,
// as artifactType.
func TestAppend_ArtifactType(t *testing.T) {
	for _, c := range []struct {
		desc, configMediaType, wantArtifactType string
	}{{
		desc:             "standard config.mediaType, no artifactType",
		configMediaType:  string(types.DockerConfigJSON),
		wantArtifactType: "",
	}, {
		desc:             "non-standard config.mediaType, want artifactType",
		configMediaType:  "application/vnd.custom.something",
		wantArtifactType: "application/vnd.custom.something",
	}} {
		t.Run(c.desc, func(t *testing.T) {
			img, err := random.Image(1, 1)
			if err != nil {
				t.Fatalf("random.Image: %v", err)
			}
			img = mutate.ConfigMediaType(img, types.MediaType(c.configMediaType))
			idx := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
				Add: img,
			})
			mf, err := idx.IndexManifest()
			if err != nil {
				t.Fatalf("IndexManifest: %v", err)
			}
			if got := mf.Manifests[0].ArtifactType; got != c.wantArtifactType {
				t.Errorf("manifest artifactType: got %q, want %q", got, c.wantArtifactType)
			}

			desc, err := partial.Descriptor(img)
			if err != nil {
				t.Fatalf("partial.Descriptor: %v", err)
			}
			if got := desc.ArtifactType; got != c.wantArtifactType {
				t.Errorf("descriptor artifactType: got %q, want %q", got, c.wantArtifactType)
			}

			gotAT, err := partial.ArtifactType(img)
			if err != nil {
				t.Fatalf("partial.ArtifactType: %v", err)
			}
			if gotAT != c.wantArtifactType {
				t.Errorf("partial.ArtifactType: got %q, want %q", gotAT, c.wantArtifactType)
			}
		})
	}
}
