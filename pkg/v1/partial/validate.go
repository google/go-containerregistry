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

package partial

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// ValidateImage takes an image and checks that it can read all of the parts.
// - for config, tries to read the config; if it can do so, it is valid
// - for layers, tries to read the layer; if it can do so, it is valid
//
// It doesn't care if the image is from remote, or local v1/layout, or anywhere.
// It just cares that it can reach it.
func ValidateImage(img v1.Image) bool {
	layers, err := img.Layers()
	if err != nil {
		return false
	}

	for _, layer := range layers {
		r, err := layer.Compressed()
		if err != nil || r == nil {
			return false
		}
		r.Close()
	}
	if _, err := img.RawConfigFile(); err != nil {
		return false
	}
	return true
}

// ValidateIndex takes an image and checks that it can read all of the parts.
// - for each manifest and index, tries to retrieve them and read them; if it can do so, it is valid
//
// It needs to actually retrieve and read a manifest and an index, so it can get the
// next layers down
//
// It doesn't care if the image is from remote, or local v1/layout, or anywhere.
// It just cares that it can reach it.
func ValidateIndex(ii v1.ImageIndex) bool {
	index, err := ii.IndexManifest()
	if err != nil {
		return false
	}

	// Walk the descriptors and check for accessibility of
	// any descriptors we find, plus their children
	for _, desc := range index.Manifests {
		switch desc.MediaType {
		case types.OCIImageIndex, types.DockerManifestList:
			ii, err := ii.ImageIndex(desc.Digest)
			if err != nil {
				return false
			}
			if !ValidateIndex(ii) {
				return false
			}
		case types.OCIManifestSchema1, types.DockerManifestSchema2:
			img, err := ii.Image(desc.Digest)
			if err != nil {
				return false
			}
			if !ValidateImage(img) {
				return false
			}
		default:
			// TODO: could reference arbitrary things, which we should
			// probably just check for access.
		}
	}
	return true
}
