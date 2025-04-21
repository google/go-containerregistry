// Copyright 2025 Google LLC All Rights Reserved.
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

package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// MockImage is a fake implementation of v1.Image for testing.
type MockImage struct {
	ConfigFileVal     *v1.ConfigFile
	DigestVal         v1.Hash
	ManifestVal       []byte
	RawConfigFileVal  []byte
	LayersVal         []v1.Layer
	MediaTypeVal      types.MediaType
	SizeVal           int64
	LayerByDigestResp v1.Layer
	LayerByDigestErr  error
}

// Layers returns the ordered collection of filesystem layers that comprise this image.
func (i *MockImage) Layers() ([]v1.Layer, error) {
	return i.LayersVal, nil
}

// MediaType returns the media type of this image's manifest.
func (i *MockImage) MediaType() (types.MediaType, error) {
	return i.MediaTypeVal, nil
}

// Size returns the size of the manifest.
func (i *MockImage) Size() (int64, error) {
	return i.SizeVal, nil
}

// ConfigName returns the hash of the image's config file, also known as the Image ID.
func (i *MockImage) ConfigName() (v1.Hash, error) {
	return i.DigestVal, nil
}

// ConfigFile returns this image's config file.
func (i *MockImage) ConfigFile() (*v1.ConfigFile, error) {
	return i.ConfigFileVal, nil
}

// RawConfigFile returns the serialized bytes of ConfigFile().
func (i *MockImage) RawConfigFile() ([]byte, error) {
	return i.RawConfigFileVal, nil
}

// Digest returns the sha256 of this image's manifest.
func (i *MockImage) Digest() (v1.Hash, error) {
	return i.DigestVal, nil
}

// Manifest returns this image's Manifest object.
func (i *MockImage) Manifest() (*v1.Manifest, error) {
	m := v1.Manifest{}
	if err := json.Unmarshal(i.ManifestVal, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// RawManifest returns the serialized bytes of Manifest().
func (i *MockImage) RawManifest() ([]byte, error) {
	return i.ManifestVal, nil
}

// LayerByDigest returns a Layer for interacting with a particular layer of
// the image, looking it up by "digest" (the compressed hash).
func (i *MockImage) LayerByDigest(_ v1.Hash) (v1.Layer, error) {
	return i.LayerByDigestResp, i.LayerByDigestErr
}

// LayerByDiffID is an analog to LayerByDigest, looking up by "diff id"
// (the uncompressed hash).
func (i *MockImage) LayerByDiffID(_ v1.Hash) (v1.Layer, error) {
	return i.LayerByDigestResp, i.LayerByDigestErr
}

// MockLayer is a fake implementation of v1.Layer for testing.
type MockLayer struct {
	DigestVal           v1.Hash
	DiffIDVal           v1.Hash
	SizeVal             int64
	MediaTypeVal        types.MediaType
	UncompressedVal     []byte
	CompressedVal       []byte
	UncompressedSizeVal int64
}

// Digest returns the Hash of the compressed layer.
func (l *MockLayer) Digest() (v1.Hash, error) {
	return l.DigestVal, nil
}

// DiffID returns the Hash of the uncompressed layer.
func (l *MockLayer) DiffID() (v1.Hash, error) {
	return l.DiffIDVal, nil
}

// Compressed returns an io.ReadCloser for the compressed layer contents.
func (l *MockLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.CompressedVal)), nil
}

// Uncompressed returns an io.ReadCloser for the uncompressed layer contents.
func (l *MockLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.UncompressedVal)), nil
}

// Size returns the compressed size of the Layer.
func (l *MockLayer) Size() (int64, error) {
	return l.SizeVal, nil
}

// MediaType returns the media type of the Layer.
func (l *MockLayer) MediaType() (types.MediaType, error) {
	return l.MediaTypeVal, nil
}

// UncompressedSize returns the uncompressed size of the Layer.
func (l *MockLayer) UncompressedSize() (int64, error) {
	return l.UncompressedSizeVal, nil
}

// PushTestImage uploads a test image to the given registry for testing.
// Returns the image reference that was pushed and any error.
func PushTestImage(t *testing.T, registry string, repo string, tag string) (string, error) {
	t.Helper()

	// Create a reference that includes the registry, repo, and tag
	ref := fmt.Sprintf("%s/%s:%s", registry, repo, tag)

	// Get an empty image from the crane package
	img, err := crane.Image(map[string][]byte{})
	if err != nil {
		return "", fmt.Errorf("failed to create empty image: %w", err)
	}

	// Push the image to the registry
	if err := crane.Push(img, ref); err != nil {
		return "", fmt.Errorf("failed to push test image: %w", err)
	}

	return ref, nil
}
