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

package crane

import (
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
)

var defaultPlatform = v1.Platform{
	Architecture: "amd64",
	OS:           "linux",
}

// matchesPlatform checks if the given platform matches the required platforms.
// The given platform matches the required platform if
// - architecture and OS are identical.
// - OS version and variant are identical if provided.
// - features and OS features of the required platform are subsets of those of the given platform.
func matchesPlatform(given, required v1.Platform) bool {
	// Required fields that must be identical.
	if given.Architecture != required.Architecture || given.OS != required.OS {
		return false
	}

	// Optional fields that may be empty, but must be identical if provided.
	if required.OSVersion != "" && given.OSVersion != required.OSVersion {
		return false
	}
	if required.Variant != "" && given.Variant != required.Variant {
		return false
	}

	// Verify required platform's features are a subset of given platform's features.
	if !isSubset(given.OSFeatures, required.OSFeatures) {
		return false
	}
	if !isSubset(given.Features, required.Features) {
		return false
	}

	return true
}

// isSubset checks if the required array of strings is a subset of the given lst.
func isSubset(lst, required []string) bool {
	set := make(map[string]bool)
	for _, value := range lst {
		set[value] = true
	}

	for _, value := range required {
		if _, ok := set[value]; !ok {
			return false
		}
	}

	return true
}

func childByPlatform(idx v1.ImageIndex, platform v1.Platform) (partial.Artifact, error) {
	index, err := idx.IndexManifest()
	if err != nil {
		return nil, err
	}
	for _, childDesc := range index.Manifests {
		// If platform is missing from child descriptor, assume it's amd64/linux.
		p := defaultPlatform
		if childDesc.Platform != nil {
			p = *childDesc.Platform
		}

		if matchesPlatform(p, platform) {
			if childDesc.MediaType.IsIndex() {
				return idx.ImageIndex(childDesc.Digest)
			} else if childDesc.MediaType.IsImage() {
				return idx.Image(childDesc.Digest)
			} else if childDesc.MediaType.IsSchema1() {
				return idx.Image(childDesc.Digest)
			}
		}
	}
	return nil, fmt.Errorf("no child with platform %+v in index", platform)
}

func GetImage(r string, opt ...Option) (v1.Image, error) {
	o := makeOptions(opt...)
	ar, err := Artifact(r, opt...)
	if err != nil {
		return nil, fmt.Errorf("reading image %q: %w", r, err)
	}

	if img, ok := ar.(v1.Image); ok {
		return img, nil
	} else if idx, ok := ar.(v1.ImageIndex); ok && o.Platform != nil {
		img, err := childByPlatform(idx, *o.Platform)
		if err != nil {
			return nil, err
		}
		if img, ok := img.(v1.Image); ok {
			return img, nil
		}
	}
	mt, err := ar.MediaType()
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%s (%s) is not an image", r, mt)
}
