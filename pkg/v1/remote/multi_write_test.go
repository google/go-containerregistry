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

package remote

import (
	"io/ioutil"
	"log"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestMultiWrite(t *testing.T) {
	// Create a random image.
	img1, err := random.Image(1024, 2)
	if err != nil {
		t.Fatal("random.Image:", err)
	}

	// Create another image that's based on the first.
	rl, err := random.Layer(1024, types.OCIUncompressedLayer)
	if err != nil {
		t.Fatal("random.Layer:", err)
	}
	img2, err := mutate.AppendLayers(img1, rl)
	if err != nil {
		t.Fatal("mutate.AppendLayers:", err)
	}

	// Also create a random index of images.
	subidx, err := random.Index(1024, 2, 3)
	if err != nil {
		t.Fatal("random.Index:", err)
	}

	// Add a sub-sub-index of random images.
	subsubidx, err := random.Index(1024, 3, 4)
	if err != nil {
		t.Fatal("random.Index:", err)
	}
	subidx = mutate.AppendManifests(subidx, mutate.IndexAddendum{Add: subsubidx})

	// Create an index containing both images and the index above.
	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: img1},
		mutate.IndexAddendum{Add: img2},
		mutate.IndexAddendum{Add: subidx},
		mutate.IndexAddendum{Add: rl},
	)

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Write both images and the manifest list.
	tag1, tag2, tag3 := mustNewTag(t, u.Host+"/repo:tag1"), mustNewTag(t, u.Host+"/repo:tag2"), mustNewTag(t, u.Host+"/repo:tag3")
	if err := MultiWrite(map[name.Reference]Taggable{
		tag1: img1,
		tag2: img2,
		tag3: idx,
	}); err != nil {
		t.Error("Write:", err)
	}

	// Check that tagged images are present.
	for _, tag := range []name.Tag{tag1, tag2} {
		got, err := Image(tag)
		if err != nil {
			t.Error(err)
			continue
		}
		if err := validate.Image(got); err != nil {
			t.Error("Validate() =", err)
		}
	}

	// Check that tagged manfest list is present and valid.
	got, err := Index(tag3)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Index(got); err != nil {
		t.Error("Validate() =", err)
	}
}

func TestMultiWriteWithNondistributableLayer(t *testing.T) {
	// Create a random image.
	img1, err := random.Image(1024, 2)
	if err != nil {
		t.Fatal("random.Image:", err)
	}

	// Create another image that's based on the first.
	rl, err := random.Layer(1024, types.OCIRestrictedLayer)
	if err != nil {
		t.Fatal("random.Layer:", err)
	}
	img, err := mutate.AppendLayers(img1, rl)
	if err != nil {
		t.Fatal("mutate.AppendLayers:", err)
	}

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Write the image.
	tag1 := mustNewTag(t, u.Host+"/repo:tag1")
	if err := MultiWrite(map[name.Reference]Taggable{tag1: img}, WithNondistributable); err != nil {
		t.Error("Write:", err)
	}

	// Check that tagged image is present.
	got, err := Image(tag1)
	if err != nil {
		t.Error(err)
	}
	if err := validate.Image(got); err != nil {
		t.Error("Validate() =", err)
	}
}

// TestMultiWrite_Deep tests that a deeply nested tree of manifest lists gets
// pushed in the correct order (i.e., each level in sequence).
func TestMultiWrite_Deep(t *testing.T) {
	idx, err := random.Index(1024, 2, 2)
	if err != nil {
		t.Fatal("random.Image:", err)
	}
	for i := 0; i < 10; i++ {
		idx = mutate.AppendManifests(idx, mutate.IndexAddendum{Add: idx})
	}

	// Set up a fake registry (with NOP logger to avoid spamming test logs).
	nopLog := log.New(ioutil.Discard, "", 0)
	s := httptest.NewServer(registry.New(registry.Logger(nopLog)))
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Write both images and the manifest list.
	tag := mustNewTag(t, u.Host+"/repo:tag")
	if err := MultiWrite(map[name.Reference]Taggable{
		tag: idx,
	}); err != nil {
		t.Error("Write:", err)
	}

	// Check that tagged manfest list is present and valid.
	got, err := Index(tag)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Index(got); err != nil {
		t.Error("Validate() =", err)
	}
}
