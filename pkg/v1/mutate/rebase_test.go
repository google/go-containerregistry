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
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
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

// TestRebaseLayerType tests that when rebasing an image, if the new base doesn't have
// the same layer type as the image to rebase, the new base layer type is used.
func TestRebaseLayerType(t *testing.T) {
	// Create an old base image with DockerLayer type
	oldBaseLayer, err := random.Layer(100, types.DockerLayer)
	if err != nil {
		t.Fatalf("random.Layer (oldBase): %v", err)
	}
	oldBase, err := mutate.AppendLayers(empty.Image, oldBaseLayer)
	if err != nil {
		t.Fatalf("mutate.AppendLayers (oldBase): %v", err)
	}

	// Create an image to rebase with OCILayer type
	origLayer, err := random.Layer(100, types.DockerLayer)
	if err != nil {
		t.Fatalf("random.Layer (orig): %v", err)
	}
	orig, err := mutate.Append(oldBase, mutate.Addendum{
		Layer: origLayer,
		History: v1.History{
			Author:    "me",
			Created:   v1.Time{Time: time.Now()},
			CreatedBy: "test",
			Comment:   "this is a test",
		},
	})
	if err != nil {
		t.Fatalf("mutate.Append (orig): %v", err)
	}

	// Create a new base image with DockerLayer type
	newBaseLayer, err := random.Layer(100, types.OCILayer)
	if err != nil {
		t.Fatalf("random.Layer (newBase): %v", err)
	}
	newBase, err := mutate.AppendLayers(empty.Image, newBaseLayer)
	if err != nil {
		t.Fatalf("mutate.AppendLayers (newBase): %v", err)
	}

	// Rebase the image onto the new base
	rebased, err := mutate.Rebase(orig, oldBase, newBase)
	if err != nil {
		t.Fatalf("mutate.Rebase: %v", err)
	}

	// Get the layers of the rebased image
	rebasedLayers, err := rebased.Layers()
	if err != nil {
		t.Fatalf("rebased.Layers: %v", err)
	}

	// Check that the rebased image has 2 layers
	if len(rebasedLayers) != 2 {
		t.Fatalf("rebased image has %d layers, expected 2", len(rebasedLayers))
	}

	// Check that the first layer (from the new base) has OCILayer type
	firstLayerType, err := rebasedLayers[0].MediaType()
	if err != nil {
		t.Fatalf("rebasedLayers[0].MediaType: %v", err)
	}
	if firstLayerType != types.OCILayer {
		t.Errorf("first layer has media type %v, expected %v", firstLayerType, types.OCILayer)
	}

	// Check that the second layer (from orig) has OCILayer type (should have been changed from DockerLayer)
	secondLayerType, err := rebasedLayers[1].MediaType()
	if err != nil {
		t.Fatalf("rebasedLayers[1].MediaType: %v", err)
	}

	// Print more information about the layers to help diagnose the issue
	t.Logf("Original layer media type: %v", types.DockerLayer)
	t.Logf("New base layer media type: %v", types.OCILayer)
	t.Logf("First rebased layer media type: %v", firstLayerType)
	t.Logf("Second rebased layer media type: %v", secondLayerType)

	// Check if the original layer is actually a DockerLayer
	origLayerType, err := origLayer.MediaType()
	if err != nil {
		t.Logf("Error getting original layer media type: %v", err)
	} else {
		t.Logf("Original layer actual media type: %v", origLayerType)
	}

	if secondLayerType != types.OCILayer {
		t.Errorf("second layer has media type %v, expected %v", secondLayerType, types.OCILayer)
	}
}
