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

package cache

import (
	"errors"
	"io"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestImage(t *testing.T) {
	img, err := random.Image(1024, 5)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	m := &memcache{map[v1.Hash]v1.Layer{}}
	img = Image(img, m)

	// Validate twice to hit the cache.
	if err := validate.Image(img); err != nil {
		t.Errorf("Validate: %v", err)
	}
	if err := validate.Image(img); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestImageIndex(t *testing.T) {
	// ImageIndex with child Image and ImageIndex manifests.
	ii, err := random.Index(1024, 5, 2)
	if err != nil {
		t.Fatalf("random.Index: %v", err)
	}
	iiChild, err := random.Index(1024, 5, 2)
	if err != nil {
		t.Fatalf("random.Index: %v", err)
	}
	ii = mutate.AppendManifests(ii, mutate.IndexAddendum{Add: iiChild})

	m := &memcache{map[v1.Hash]v1.Layer{}}
	ii = ImageIndex(ii, m)

	// Validate twice to hit the cache.
	if err := validate.Index(ii); err != nil {
		t.Errorf("Validate: %v", err)
	}
	if err := validate.Index(ii); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestLayersLazy(t *testing.T) {
	img, err := random.Image(1024, 5)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	m := &memcache{map[v1.Hash]v1.Layer{}}
	img = Image(img, m)

	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("img.Layers: %v", err)
	}

	// After calling Layers, nothing is cached.
	if got, want := len(m.m), 0; got != want {
		t.Errorf("Cache has %d entries, want %d", got, want)
	}

	rc, err := layers[0].Uncompressed()
	if err != nil {
		t.Fatalf("layer.Uncompressed: %v", err)
	}
	io.Copy(io.Discard, rc)

	if got, expected := len(m.m), 1; got != expected {
		t.Errorf("expected %v layers in cache after reading, got %v", expected, got)
	}
}

// TestCacheShortCircuit tests that if a layer is found in the cache,
// LayerByDigest is not called in the underlying Image implementation.
func TestCacheShortCircuit(t *testing.T) {
	l := &fakeLayer{}
	m := &memcache{map[v1.Hash]v1.Layer{
		fakeHash: l,
	}}
	img := Image(&fakeImage{}, m)

	for i := 0; i < 10; i++ {
		if _, err := img.LayerByDigest(fakeHash); err != nil {
			t.Errorf("LayerByDigest[%d]: %v", i, err)
		}
	}
}

var fakeHash = v1.Hash{Algorithm: "fake", Hex: "data"}

type fakeLayer struct{ v1.Layer }
type fakeImage struct{ v1.Image }

func (f *fakeImage) LayerByDigest(v1.Hash) (v1.Layer, error) {
	return nil, errors.New("LayerByDigest was called")
}

// memcache is an in-memory Cache implementation.
//
// It doesn't intend to actually write layer data, it just keeps a reference
// to the original Layer.
//
// It only assumes/considers compressed layers, and so only writes layers by
// digest.
type memcache struct {
	m map[v1.Hash]v1.Layer
}

func (m *memcache) Put(l v1.Layer) (v1.Layer, error) {
	digest, err := l.Digest()
	if err != nil {
		return nil, err
	}
	m.m[digest] = l
	return l, nil
}

func (m *memcache) Get(h v1.Hash) (v1.Layer, error) {
	l, found := m.m[h]
	if !found {
		return nil, ErrNotFound
	}
	return l, nil
}

func (m *memcache) Delete(h v1.Hash) error {
	delete(m.m, h)
	return nil
}
