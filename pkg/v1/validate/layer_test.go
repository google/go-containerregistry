// Copyright 2019 Google LLC All Rights Reserved.
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

package validate

import (
	"bytes"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"io"
	"testing"
)

// Tests that validation succeeds for valid layer that contains GZ-compressed tar content.
func TestValidateCompressedLayer(t *testing.T) {
	layer, err := random.Layer(100, types.OCILayer)
	if err != nil {
		t.Error("could not create layer:", err)
	}
	if err := Layer(layer); err != nil {
		t.Error("compressed layer failed validation:", err)
	}
}

// Tests that validation succeeds for valid layer that contains uncompressed tar content.
func TestValidateLayerWithTarContentNotCompressed(t *testing.T) {
	layer, err := random.Layer(100, types.DockerUncompressedLayer)
	if err != nil {
		t.Error("could not create layer:", err)
	}
	tarReadCloser, err := layer.Uncompressed()
	if err != nil {
		t.Error("could not get tar content:", err)
	}
	tarBytes, err := io.ReadAll(tarReadCloser)
	if err != nil {
		t.Error("could not read tar content:", err)
	}
	layer, err = partial.CompressedToLayer(&compressedBytesLayer{
		content: tarBytes,
	})
	if err != nil {
		t.Error("could not create layer:", err)
	}
	if err := Layer(layer); err != nil {
		t.Error("compressed layer failed validation:", err)
	}
}

// Tests that validation succeeds for valid layer that contains uncompressed arbitrary content.
func TestValidateLayerWithContentNotCompressed(t *testing.T) {
	layer, err := partial.CompressedToLayer(&compressedBytesLayer{
		content: []byte("test-content"),
	})
	if err != nil {
		t.Error("could not create layer:", err)
	}
	if err := Layer(layer); err != nil {
		t.Error("compressed layer failed validation:", err)
	}
}

type compressedBytesLayer struct {
	content []byte
}

func (cl *compressedBytesLayer) Digest() (v1.Hash, error) {
	hash, _, err := v1.SHA256(bytes.NewReader(cl.content))
	return hash, err
}

func (cl *compressedBytesLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(cl.content)), nil
}

func (cl *compressedBytesLayer) Size() (int64, error) {
	return int64(len(cl.content)), nil
}

func (cl *compressedBytesLayer) MediaType() (types.MediaType, error) {
	return types.OCIContentDescriptor, nil
}
