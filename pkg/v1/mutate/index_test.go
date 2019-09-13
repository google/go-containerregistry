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
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
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
			URLs: []string{"layer.example.com"},
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
		5: "layer.example.com",
	} {
		if got := m.Manifests[i].URLs[0]; got != want {
			t.Errorf("wrong URLs[0] for Manifests[%d]: %s != %s", i, got, want)
		}
	}

	if got, want := m.Manifests[5].MediaType, types.OCIUncompressedRestrictedLayer; got != want {
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
