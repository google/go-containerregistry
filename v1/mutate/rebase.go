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
	"log"
	"strings"

	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/empty"
)

type RebaseOptions struct {
	NewBaseTag    name.Tag
	NewBaseDigest name.Digest
}

func Rebase(orig, oldBase, newBase v1.Image, opts *RebaseOptions) (v1.Image, error) {
	// Verify that oldBase's layers are present in orig, otherwise orig is
	// not based on oldBase at all.
	origLayers, err := orig.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers for original: %v", err)
	}
	oldBaseLayers, err := oldBase.Layers()
	if err != nil {
		return nil, err
	}
	if len(oldBaseLayers) > len(origLayers) {
		return nil, fmt.Errorf("image %q is not based on %q", orig, oldBase)
	}
	for i, l := range oldBaseLayers {
		oldLayerDigest, _ := l.Digest()
		origLayerDigest, _ := origLayers[i].Digest()
		if oldLayerDigest != origLayerDigest {
			return nil, fmt.Errorf("image %q is not based on %q", orig, oldBase)
		}
	}

	origConfig, err := orig.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get config for original: %v", err)
	}

	// Stitch together an image that contains:
	// - original image's config
	// - new base image's layers + top of original image's layers
	// - new base image's history + top of original image's history
	rebasedConfig := *origConfig.Config.DeepCopy()
	if opts != nil {
		if rebasedConfig.Labels == nil {
			rebasedConfig.Labels = map[string]string{}
		}
		rebasedConfig.Labels["rebase"] = fmt.Sprintf("%s %s", opts.NewBaseDigest, opts.NewBaseTag)
		log.Println("Adding LABEL rebase", rebasedConfig.Labels["rebase"])
	}
	rebasedImage, err := Config(empty.Image, rebasedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create empty image with original config: %v", err)
	}
	// Get new base layers and config for history.
	newBaseLayers, err := newBase.Layers()
	if err != nil {
		return nil, fmt.Errorf("could not get new base layers for new base: %v", err)
	}
	newConfig, err := newBase.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("could not get config for new base: %v", err)
	}
	for i := range newBaseLayers {
		rebasedImage, err = Append(rebasedImage, Addendum{
			Layer:   newBaseLayers[i],
			History: newConfig.History[i],
		})
		if err != nil {
			return nil, fmt.Errorf("failed to append layer %d of new base layers", i)
		}
	}
	for i := range origLayers[len(oldBaseLayers):] {
		rebasedImage, err = Append(rebasedImage, Addendum{
			Layer:   origLayers[i],
			History: origConfig.History[i],
		})
		if err != nil {
			return nil, fmt.Errorf("failed to append layer %d of original layers", i)
		}
	}
	return rebasedImage, nil
}

type RebaseHint struct {
	OldBase, NewBase string
}

func DetectRebaseHint(img v1.Image) (*RebaseHint, error) {
	config, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}
	lbls := config.Config.Labels
	if lbls == nil {
		return nil, nil
	}
	lbl, found := lbls["rebase"]
	if !found {
		return nil, nil
	}
	parts := strings.Split(lbl, " ")
	if len(parts) < 2 {
		return nil, fmt.Errorf("Malformed rebase LABEL: %s", lbl)
	}
	return &RebaseHint{parts[0], parts[1]}, nil
}
