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

package partial

import (
	"bytes"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

var defaultPlatform = v1.Platform{
	Architecture: "amd64",
	OS:           "linux",
}

// WithRawIndexManifest defines the subset of v1.ImageIndex used by ImageByArch.
type WithRawIndexManifest interface {
	RawManifest() ([]byte, error)
	Image(v1.Hash) (v1.Image, error)
}

// IndexManifest is a helper for implementing v1.ImageIndex.
func IndexManifest(w WithRawIndexManifest) (*v1.IndexManifest, error) {
	b, err := w.RawManifest()
	if err != nil {
		return nil, err
	}
	return v1.ParseIndexManifest(bytes.NewReader(b))
}

// ImageByPlatform returns the v1.Image belonging to the v1.ImageIndex with the
// matching Platform specification.
func ImageByPlatform(w WithRawIndexManifest, p v1.Platform) (v1.Image, error) {
	im, err := IndexManifest(w)
	if err != nil {
		return nil, err
	}
	for _, manifest := range im.Manifests {
		mp := defaultPlatform
		if manifest.Platform != nil {
			mp = *manifest.Platform
		}
		if matchesPlatform(mp, p) {
			return w.Image(manifest.Digest)
		}
	}
	return nil, fmt.Errorf("failed to find image with platform %s/%s in index", p.OS, p.Architecture)
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
