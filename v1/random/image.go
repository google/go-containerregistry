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

package random

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/partial"
	"github.com/google/go-containerregistry/v1/types"
	"github.com/google/go-containerregistry/v1/v1util"
)

// Image returns a pseudo-randomly generated Image.
func Image(byteSize, layers int64) (v1.Image, error) {
	layerz := make(map[v1.Hash][]byte)
	for i := int64(0); i < layers; i++ {
		b := bytes.NewBuffer(nil)
		if _, err := io.CopyN(b, rand.Reader, byteSize); err != nil {
			return nil, err
		}
		bts := b.Bytes()
		h, _, err := v1.SHA256(v1util.NopReadCloser(bytes.NewBuffer(bts)))
		if err != nil {
			return nil, err
		}
		layerz[h] = bts
	}

	cfg := &v1.ConfigFile{}

	// It is ok that iteration order is random in Go, because this is the random image anyways.
	for k := range layerz {
		cfg.RootFS.DiffIDs = append(cfg.RootFS.DiffIDs, k)
	}

	return partial.UncompressedToImage(&image{
		config: cfg,
		layers: layerz,
	})
}

// image is pseudo-randomly generated.
type image struct {
	config *v1.ConfigFile
	layers map[v1.Hash][]byte
}

var _ partial.UncompressedImageCore = (*image)(nil)

// RawConfigFile implements partial.UncompressedImageCore
func (i *image) RawConfigFile() ([]byte, error) {
	return partial.RawConfigFile(i)
}

// ConfigFile implements v1.Image
func (i *image) ConfigFile() (*v1.ConfigFile, error) {
	return i.config, nil
}

// MediaType implements partial.UncompressedImageCore
func (i *image) MediaType() (types.MediaType, error) {
	return types.DockerManifestSchema2, nil
}

// UncompressedLayer implements partial.UncompressedImageCore
func (i *image) UncompressedLayer(diffID v1.Hash) (io.ReadCloser, error) {
	b, ok := i.layers[diffID]
	if !ok {
		return nil, fmt.Errorf("unknown diff_id: %v", diffID)
	}
	return v1util.NopReadCloser(bytes.NewBuffer(b)), nil
}
