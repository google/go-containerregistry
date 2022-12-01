// Copyright 2020 Google LLC All Rights Reserved.
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

package partial_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestFindManifests(t *testing.T) {
	ii, err := random.Index(100, 5, 6) // random image of 6 manifests, each having 5 layers of size 100
	if err != nil {
		t.Fatal("could not create random index:", err)
	}
	m, _ := ii.IndexManifest()
	digest := m.Manifests[0].Digest

	matcher := func(desc v1.Descriptor) bool {
		return desc.Digest != digest
	}

	descriptors, err := partial.FindManifests(ii, matcher)
	expected := len(m.Manifests) - 1
	switch {
	case err != nil:
		t.Error("unexpected error:", err)
	case len(descriptors) != expected:
		t.Errorf("failed on manifests, actual %d, expected %d", len(descriptors), expected)
	}
}

func TestFindImages(t *testing.T) {
	// create our imageindex with which to work
	ii, err := random.Index(100, 5, 6) // random image of 6 manifests, each having 5 layers of size 100
	if err != nil {
		t.Fatal("could not create random index:", err)
	}
	m, _ := ii.IndexManifest()
	digest := m.Manifests[0].Digest

	matcher := func(desc v1.Descriptor) bool {
		return desc.Digest != digest
	}
	images, err := partial.FindImages(ii, matcher)
	expected := len(m.Manifests) - 1
	switch {
	case err != nil:
		t.Error("unexpected error:", err)
	case len(images) != expected:
		t.Errorf("failed on images, actual %d, expected %d", len(images), expected)
	}
}

func TestFindIndexes(t *testing.T) {
	// there is no utility to generate an index of indexes, so we need to create one
	// base index
	var (
		indexCount = 5
		imageCount = 7
	)
	base := empty.Index
	// we now have 5 indexes and 5 images, so wrap them into a single index
	adds := []mutate.IndexAddendum{}
	for i := 0; i < indexCount; i++ {
		ii, err := random.Index(100, 1, 1)
		if err != nil {
			t.Fatalf("%d: unable to create random index: %v", i, err)
		}
		adds = append(adds, mutate.IndexAddendum{
			Add: ii,
			Descriptor: v1.Descriptor{
				MediaType: types.OCIImageIndex,
			},
		})
	}
	for i := 0; i < imageCount; i++ {
		img, err := random.Image(100, 1)
		if err != nil {
			t.Fatalf("%d: unable to create random image: %v", i, err)
		}
		adds = append(adds, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				MediaType: types.OCIManifestSchema1,
			},
		})
	}

	// just see if it finds all of the indexes
	matcher := func(desc v1.Descriptor) bool {
		return true
	}
	index := mutate.AppendManifests(base, adds...)
	idxes, err := partial.FindIndexes(index, matcher)
	switch {
	case err != nil:
		t.Error("unexpected error:", err)
	case len(idxes) != indexCount:
		t.Errorf("failed on index, actual %d, expected %d", len(idxes), indexCount)
	}
}
