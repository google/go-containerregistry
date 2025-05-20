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

package mutate

import (
	"fmt"
	"github.com/google/go-containerregistry/pkg/v1/types"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
)

// Rebase returns a new v1.Image where the oldBase in orig is replaced by newBase.
func Rebase(orig, oldBase, newBase v1.Image) (v1.Image, error) {
	// Verify that oldBase's layers are present in orig, otherwise orig is
	// not based on oldBase at all.
	origLayers, err := orig.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers for original: %w", err)
	}
	oldBaseLayers, err := oldBase.Layers()
	if err != nil {
		return nil, err
	}
	if len(oldBaseLayers) > len(origLayers) {
		return nil, fmt.Errorf("image %q is not based on %q (too few layers)", orig, oldBase)
	}
	for i, l := range oldBaseLayers {
		oldLayerDigest, err := l.Digest()
		if err != nil {
			return nil, fmt.Errorf("failed to get digest of layer %d of %q: %w", i, oldBase, err)
		}
		origLayerDigest, err := origLayers[i].Digest()
		if err != nil {
			return nil, fmt.Errorf("failed to get digest of layer %d of %q: %w", i, orig, err)
		}
		if oldLayerDigest != origLayerDigest {
			return nil, fmt.Errorf("image %q is not based on %q (layer %d mismatch)", orig, oldBase, i)
		}
	}

	oldConfig, err := oldBase.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get config for old base: %w", err)
	}

	origConfig, err := orig.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get config for original: %w", err)
	}

	newConfig, err := newBase.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("could not get config for new base: %w", err)
	}

	// Stitch together an image that contains:
	// - original image's config
	// - new base image's os/arch properties
	// - new base image's layers + top of original image's layers
	// - new base image's history + top of original image's history
	rebasedImage, err := Config(empty.Image, *origConfig.Config.DeepCopy())
	if err != nil {
		return nil, fmt.Errorf("failed to create empty image with original config: %w", err)
	}

	// Add new config properties from existing images.
	rebasedConfig, err := rebasedImage.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("could not get config for rebased image: %w", err)
	}
	// OS/Arch properties from new base
	rebasedConfig.Architecture = newConfig.Architecture
	rebasedConfig.OS = newConfig.OS
	rebasedConfig.OSVersion = newConfig.OSVersion

	// Apply config properties to rebased.
	rebasedImage, err = ConfigFile(rebasedImage, rebasedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to replace config for rebased image: %w", err)
	}

	// Get new base layers and config for history.
	newBaseLayers, err := newBase.Layers()
	if err != nil {
		return nil, fmt.Errorf("could not get new base layers for new base: %w", err)
	}

	// Find the last DockerLayer or OCILayer type in the new base image
	var newBaseLastLayerType = lastDockerOrOciLayerType(newBaseLayers)

	// If no layer type was found in the new base, check the original image
	if newBaseLastLayerType == "" {
		newBaseLastLayerType = lastDockerOrOciLayerType(origLayers)
	}

	// Add new base layers.
	rebasedImage, err = Append(rebasedImage, createAddendums(0, 0, newConfig.History, newBaseLayers, newBaseLastLayerType)...)
	if err != nil {
		return nil, fmt.Errorf("failed to append new base image: %w", err)
	}

	// Add original layers above the old base.
	rebasedImage, err = Append(rebasedImage, createAddendums(len(oldConfig.History), len(oldBaseLayers)+1, origConfig.History, origLayers, newBaseLastLayerType)...)
	if err != nil {
		return nil, fmt.Errorf("failed to append original image: %w", err)
	}

	return rebasedImage, nil
}

// createAddendums makes a list of addendums from a history and layers starting from a specific history and layer
// indexes.
func createAddendums(startHistory, startLayer int, history []v1.History, layers []v1.Layer, layerType types.MediaType) []Addendum {
	var adds []Addendum
	// History should be a superset of layers; empty layers (e.g. ENV statements) only exist in history.
	// They cannot be iterated identically but must be walked independently, only advancing the iterator for layers
	// when a history entry for a non-empty layer is seen.
	layerIndex := 0
	for historyIndex := range history {
		var layer v1.Layer
		emptyLayer := history[historyIndex].EmptyLayer
		if !emptyLayer {
			layer = layers[layerIndex]
			layerIndex++
		}
		if historyIndex >= startHistory || layerIndex >= startLayer {
			adds = append(adds, Addendum{
				Layer:   layerWithNormalizedMediaType(layer, layerType),
				History: history[historyIndex],
			})
		}
	}
	// In the event history was malformed or non-existent, append the remaining layers.
	for i := layerIndex; i < len(layers); i++ {
		if i >= startLayer {
			adds = append(adds, Addendum{
				Layer: layerWithNormalizedMediaType(layers[layerIndex], layerType),
			})
		}
	}

	return adds
}

func lastDockerOrOciLayerType(fromLayers []v1.Layer) types.MediaType {
	for i := len(fromLayers) - 1; i >= 0; i-- {
		mediaType, err := fromLayers[i].MediaType()
		if err == nil && isDockerOrOciLayerType(mediaType) {
			return mediaType
		}
	}
	return ""
}

func isDockerOrOciLayerType(mediaType types.MediaType) bool {
	return mediaType == types.DockerLayer || mediaType == types.OCILayer
}

// mediaTypeOverridingLayer wraps a layer and overrides its MediaType method.
type mediaTypeOverridingLayer struct {
	v1.Layer
	mediaType types.MediaType
}

// MediaType implements v1.Layer
func (l *mediaTypeOverridingLayer) MediaType() (types.MediaType, error) {
	var mt = l.mediaType
	var err error = nil
	if mt == "" {
		mt, err = l.Layer.MediaType()
	}
	return mt, err
}

func layerWithNormalizedMediaType(layer v1.Layer, layerType types.MediaType) v1.Layer {
	// Check the layer's media type
	if layer != nil && layerType != "" {
		mediaType, err := layer.MediaType()
		// Only modify the media type if it's DockerLayer or OCILayer
		if err == nil && isDockerOrOciLayerType(mediaType) {
			// Wrap the layer to override its MediaType method
			return &mediaTypeOverridingLayer{
				Layer:     layer,
				mediaType: layerType,
			}
		}
	}
	return layer
}
