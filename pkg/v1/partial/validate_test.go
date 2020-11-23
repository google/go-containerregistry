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

package partial_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
)

func testValidateCreateCache(cache string) (layout.Path, error) {
	// initialize the cache path if needed
	p, err := layout.FromPath(cache)
	if err != nil {
		p, err = layout.Write(cache, empty.Index)
		if err != nil {
			return p, err
		}
	}

	completeImage, err := random.Image(100, 5)
	if err != nil {
		return p, err
	}
	missingConfig, err := random.Image(100, 5)
	if err != nil {
		return p, err
	}
	missingLayer, err := random.Image(100, 5)
	if err != nil {
		return p, err
	}
	if err := p.AppendImage(completeImage, layout.WithAnnotations(map[string]string{
		imagespec.AnnotationRefName: "image-complete",
	})); err != nil {
		return p, err
	}
	if err := p.AppendImage(missingConfig, layout.WithAnnotations(map[string]string{
		imagespec.AnnotationRefName: "image-missing-config",
	})); err != nil {
		return p, err
	}
	if err := p.AppendImage(missingLayer, layout.WithAnnotations(map[string]string{
		imagespec.AnnotationRefName: "image-missing-layer",
	})); err != nil {
		return p, err
	}
	// now we remove some blobs
	layers, err := missingLayer.Layers()
	if err != nil {
		return p, err
	}
	if len(layers) < 1 {
		return p, errors.New("partial was missing layers")
	}
	h, err := layers[0].Digest()
	if err != nil {
		return p, fmt.Errorf("could not get hash for first layer: %v", err)
	}
	layerFile := path.Join(cache, "blobs/sha256", h.Hex)
	if err := os.Remove(layerFile); err != nil {
		return p, fmt.Errorf("failed to remove layer file: %s", layerFile)
	}
	config, err := missingConfig.ConfigName()
	if err != nil {
		return p, fmt.Errorf("could not get hash for config file: %v", err)
	}
	configFile := path.Join(cache, "blobs/sha256", config.Hex)
	if err := os.Remove(configFile); err != nil {
		return p, fmt.Errorf("failed to remove config file: %s", configFile)
	}

	completeIndex, err := random.Index(100, 5, 4)
	if err != nil {
		return p, err
	}
	missingIndex, err := random.Index(100, 5, 4)
	if err != nil {
		return p, err
	}
	if err := p.AppendIndex(completeIndex, layout.WithAnnotations(map[string]string{
		imagespec.AnnotationRefName: "index-complete",
	})); err != nil {
		return p, err
	}
	if err := p.AppendIndex(missingIndex, layout.WithAnnotations(map[string]string{
		imagespec.AnnotationRefName: "index-missing",
	})); err != nil {
		return p, err
	}
	// just remove one of them
	manifest, err := missingIndex.IndexManifest()
	if err != nil {
		return p, err
	}
	if len(manifest.Manifests) < 2 {
		return p, fmt.Errorf("index did not have at least 2 manifests, instead: %d", len(manifest.Manifests))
	}
	manifestFile := path.Join(cache, "blobs/sha256", manifest.Manifests[1].Digest.Hex)
	if err := os.Remove(manifestFile); err != nil {
		return p, fmt.Errorf("failed to remove manifest file: %s", manifestFile)
	}

	return p, nil
}

func TestValidateImage(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "validate_test")
	if err != nil {
		t.Fatal("creating temp dir", err)
	}
	defer os.RemoveAll(tmpdir)
	p, err := testValidateCreateCache(tmpdir)
	if err != nil {
		t.Fatal("error creating cache", err)
	}
	tests := []struct {
		name  string
		valid bool
	}{
		{"image-complete", true},
		{"image-missing-layer", false},
		{"image-missing-config", false},
	}
	for _, tt := range tests {
		rootIndex, err := p.ImageIndex()
		// of there is no root index, we are broken
		if err != nil {
			t.Fatal("invalid image cache", err)
		}

		images, err := partial.FindImages(rootIndex, match.Name(tt.name))
		if err != nil || len(images) < 1 {
			t.Errorf("could not find image %s: %v", tt.name, err)
		}
		image := images[0]
		valid := partial.ValidateImage(image)
		if valid != tt.valid {
			t.Errorf("mismatch for %s: actual %v expected %v", tt.name, valid, tt.valid)
		}
	}
}

func TestValidateIndex(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "validate_test")
	if err != nil {
		t.Fatal("creating temp dir", err)
	}
	defer os.RemoveAll(tmpdir)
	p, err := testValidateCreateCache(tmpdir)
	if err != nil {
		t.Fatal("error creating cache", err)
	}
	tests := []struct {
		name  string
		valid bool
	}{
		{"index-complete", true},
		{"index-missing", false},
	}
	for _, tt := range tests {
		rootIndex, err := p.ImageIndex()
		// of there is no root index, we are broken
		if err != nil {
			t.Fatal("invalid image cache", err)
		}

		indexes, err := partial.FindIndexes(rootIndex, match.Name(tt.name))
		if err != nil || len(indexes) < 1 {
			t.Fatalf("could not find index %s: %v", tt.name, err)
		}
		index := indexes[0]
		valid := partial.ValidateIndex(index)
		if valid != tt.valid {
			t.Errorf("mismatch for %s: actual %v expected %v", tt.name, valid, tt.valid)
		}
	}
}
