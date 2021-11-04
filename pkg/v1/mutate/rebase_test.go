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
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
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
		t.Log(dig)
		layerDigests[i] = dig.String()
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
	t.Log("Old base:")
	_ = layerDigests(t, oldBase)

	// Construct an image with 2 layers on top of oldBase (an empty layer and a random layer).
	top, err := random.Image(100, 1)
	if err != nil {
		t.Fatalf("random.Image (top): %v", err)
	}
	topLayers, err := top.Layers()
	if err != nil {
		t.Fatalf("top.Layers: %v", err)
	}
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
			Layer: topLayers[0],
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

	t.Log("Original:")
	origLayerDigests := layerDigests(t, orig)

	// Create a random new base image of 3 layers.
	newBase, err := random.Image(100, 3)
	if err != nil {
		t.Fatalf("random.Image (newBase): %v", err)
	}
	t.Log("New base:")
	newBaseLayerDigests := layerDigests(t, newBase)

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
	t.Log("Rebased image layer digests:")
	for i, l := range rebasedBaseLayers {
		dig, err := l.Digest()
		if err != nil {
			t.Fatalf("layer.Digest (rebased base layer %d): %v", i, err)
		}
		t.Log(dig)
		rebasedLayerDigests[i] = dig.String()
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
