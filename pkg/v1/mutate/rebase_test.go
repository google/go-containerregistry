// Copyright 2018 Google LLC All Rights Reserved.
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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func layerDigests(t *testing.T, img v1.Image) []string {
	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("oldBase.Layers: %v", err)
	}
	layerDigests := make([]string, len(layers))
	for i, l := range layers {
		dig, err := l.Digest()
		if err != nil {
			t.Fatalf("layer.Digest %d: %v", i, err)
		}
		layerDigests = append(layerDigests, dig.String())
	}
	return layerDigests
}

// TestRebase tests that layer digests are expected when performing a rebase on
// random.Image layers.
func TestRebase(t *testing.T) {
	// Create a random old base image of 5 layers and get those layers' digests.
	const oldBaseLayerCount = 5
	oldBase, err := random.Image(100, oldBaseLayerCount)
	if err != nil {
		t.Fatalf("random.Image (oldBase): %v", err)
	}
	t.Log("Old base:", layerDigests(t, oldBase))

	// Construct an image with 2 layers on top of oldBase (an empty layer and a random layer).
	orig, err := mutate.Append(oldBase,
		mutate.Addendum{
			Layer: nil,
			History: v1.History{
				Author:     "me",
				Created:    v1.Time{Time: time.Now()},
				CreatedBy:  "test-empty",
				Comment:    "this is an empty test",
				EmptyLayer: true,
			},
		},
		mutate.Addendum{
			Layer: mustLayer(t),
			History: v1.History{
				Author:    "me",
				Created:   v1.Time{Time: time.Now()},
				CreatedBy: "test",
				Comment:   "this is a test",
			},
		},
	)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	origLayerDigests := layerDigests(t, orig)
	t.Log("Original:", origLayerDigests)

	// Create a random new base image of 3 layers.
	newBase, err := random.Image(100, 3)
	if err != nil {
		t.Fatalf("random.Image (newBase): %v", err)
	}
	newBaseLayerDigests := layerDigests(t, newBase)
	t.Log("New base:", newBaseLayerDigests)

	// Add config file os/arch property fields
	newBaseConfigFile, err := newBase.ConfigFile()
	if err != nil {
		t.Fatalf("newBase.ConfigFile: %v", err)
	}
	newBaseConfigFile.Architecture = "arm"
	newBaseConfigFile.OS = "windows"
	newBaseConfigFile.OSVersion = "10.0.17763.1339"

	newBase, err = mutate.ConfigFile(newBase, newBaseConfigFile)
	if err != nil {
		t.Fatalf("ConfigFile (newBase): %v", err)
	}

	// Rebase original image onto new base.
	rebased, err := mutate.Rebase(orig, oldBase, newBase)
	if err != nil {
		t.Fatalf("Rebase: %v", err)
	}

	rebasedBaseLayers, err := rebased.Layers()
	if err != nil {
		t.Fatalf("rebased.Layers: %v", err)
	}
	rebasedLayerDigests := make([]string, len(rebasedBaseLayers))
	for i, l := range rebasedBaseLayers {
		dig, err := l.Digest()
		if err != nil {
			t.Fatalf("layer.Digest (rebased base layer %d): %v", i, err)
		}
		rebasedLayerDigests = append(rebasedLayerDigests, dig.String())
	}

	// Compare rebased layers.
	wantLayerDigests := append(newBaseLayerDigests, origLayerDigests[len(origLayerDigests)-1])
	if len(rebasedLayerDigests) != len(wantLayerDigests) {
		t.Fatalf("Rebased image contained %d layers, want %d", len(rebasedLayerDigests), len(wantLayerDigests))
	}
	for i, rl := range rebasedLayerDigests {
		if got, want := rl, wantLayerDigests[i]; got != want {
			t.Errorf("Layer %d mismatch, got %q, want %q", i, got, want)
		}
	}

	// Compare rebased history.
	origConfig, err := orig.ConfigFile()
	if err != nil {
		t.Fatalf("orig.ConfigFile: %v", err)
	}
	newBaseConfig, err := newBase.ConfigFile()
	if err != nil {
		t.Fatalf("newBase.ConfigFile: %v", err)
	}
	rebasedConfig, err := rebased.ConfigFile()
	if err != nil {
		t.Fatalf("rebased.ConfigFile: %v", err)
	}
	wantHistories := append(newBaseConfig.History, origConfig.History[oldBaseLayerCount:]...)
	if len(wantHistories) != len(rebasedConfig.History) {
		t.Fatalf("Rebased image contained %d history, want %d", len(rebasedConfig.History), len(wantHistories))
	}
	for i, rh := range rebasedConfig.History {
		if got, want := rh.Comment, wantHistories[i].Comment; got != want {
			t.Errorf("Layer %d mismatch, got %q, want %q", i, got, want)
		}
	}

	// Compare ConfigFile property fields copied from new base.
	if rebasedConfig.Architecture != newBaseConfig.Architecture {
		t.Errorf("ConfigFile property Architecture mismatch, got %q, want %q", rebasedConfig.Architecture, newBaseConfig.Architecture)
	}
	if rebasedConfig.OS != newBaseConfig.OS {
		t.Errorf("ConfigFile property OS mismatch, got %q, want %q", rebasedConfig.OS, newBaseConfig.OS)
	}
	if rebasedConfig.OSVersion != newBaseConfig.OSVersion {
		t.Errorf("ConfigFile property OSVersion mismatch, got %q, want %q", rebasedConfig.OSVersion, newBaseConfig.OSVersion)
	}
}

func TestRebaseIndex(t *testing.T) {
	plats := []v1.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm", Variant: "v3", OSVersion: "six", Features: []string{"power windows"}, OSFeatures: []string{"anti-lock brakes"}},
		{OS: "linux", Architecture: "arm64"},
	}
	// Generate some more platforms.
	for i := 0; i < 10; i++ {
		plats = append(plats, v1.Platform{OS: "fakeos", Architecture: fmt.Sprintf("v%d", i)})
	}

	// Construct an old base image index containing random images.
	var oldBase v1.ImageIndex = empty.Index
	for _, plat := range plats {
		plat := plat
		img, err := random.Image(100, 3)
		if err != nil {
			t.Fatal(err)
		}
		oldBase = mutate.AppendManifests(oldBase, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				MediaType: types.DockerManifestSchema2,
				Platform:  &plat,
			},
		})
	}

	// Construct the new image, with images based on the old base image index's images.
	var orig v1.ImageIndex = empty.Index
	oldmf, err := oldBase.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range oldmf.Manifests {
		img, err := oldBase.Image(d.Digest)
		if err != nil {
			t.Fatal(err)
		}
		img, err = mutate.AppendLayers(img, mustLayer(t), mustLayer(t))
		if err != nil {
			t.Fatal(err)
		}
		orig = mutate.AppendManifests(orig, mutate.IndexAddendum{
			Add:        img,
			Descriptor: d,
		})
	}

	// Construct a new base image containing random images.
	var newBase v1.ImageIndex = empty.Index
	for _, plat := range append(
		[]v1.Platform{{OS: "windows", Architecture: "gothic revival"}}, // New OS+arch; will be ignored.
		plats...) {
		plat := plat

		img, err := random.Image(3, 10)
		if err != nil {
			t.Fatal(err)
		}
		newBase = mutate.AppendManifests(newBase, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				MediaType: types.DockerManifestSchema2,
				Platform:  &plat,
			},
		})
	}

	got, err := mutate.RebaseIndex(orig, oldBase, newBase)
	if err != nil {
		t.Fatalf("RebaseIndex: %v", err)
	}

	if err := validate.Index(got); err != nil {
		t.Errorf("validate.Index: %v", err)
	}

	// Set of platforms is as expected.
	var gotPlats []v1.Platform
	gotmf, err := got.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range gotmf.Manifests {
		gotPlats = append(gotPlats, *d.Platform)
	}
	if d := cmp.Diff(plats, gotPlats); d != "" {
		t.Errorf("Platform diffs (-want,+got): %s", d)
	}

	// Each image in the index is based on the corresponding
	// platform-specific image in newBase.
	for _, plat := range plats {
		base := layerDigests(t, imageByPlatform(t, newBase, plat))
		img := layerDigests(t, imageByPlatform(t, got, plat))

		if len(base) >= len(img) {
			t.Errorf("For platform (%v), base image had %d layers, rebased had %d", plat, len(base), len(img))
			continue
		}
		gotBase := img[:len(base)]
		if d := cmp.Diff(base, gotBase); d != "" {
			t.Errorf("For platform (%v), got base image diff (-want,+got): %s", plat, d)
		}
	}
}

func imageByPlatform(t *testing.T, idx v1.ImageIndex, p v1.Platform) v1.Image {
	mf, err := idx.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range mf.Manifests {
		if d.Platform.Equals(p) {
			img, err := idx.Image(d.Digest)
			if err != nil {
				t.Fatal(err)
			}
			return img
		}
	}
	t.Fatalf("could not find image for platform %v", p)
	return nil
}

func mustLayer(t *testing.T) v1.Layer {
	l, err := random.Layer(100, types.OCILayer)
	if err != nil {
		t.Fatal(err)
	}
	return l
}
