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

package layout

import (
	"fmt"
	"io"
	"sync"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type layoutImage struct {
	path         string
	desc         v1.Descriptor
	manifestLock sync.Mutex // Protects rawManifest
	rawManifest  []byte
}

var _ partial.CompressedImageCore = (*layoutImage)(nil)

// Image reads a v1.Image from the OCI image layout at path with digest h.
func Image(path string, h v1.Hash) (v1.Image, error) {
	// Read the index.json so we can find the manifest descriptor.
	ii, err := Index(path)
	if err != nil {
		return nil, err
	}

	return ii.Image(h)
}

func (li *layoutImage) MediaType() (types.MediaType, error) {
	return li.desc.MediaType, nil
}

// Implements WithManifest for partial.Blobset.
func (li *layoutImage) Manifest() (*v1.Manifest, error) {
	return partial.Manifest(li)
}

func (li *layoutImage) RawManifest() ([]byte, error) {
	li.manifestLock.Lock()
	defer li.manifestLock.Unlock()
	if li.rawManifest != nil {
		return li.rawManifest, nil
	}

	b, err := Bytes(li.path, li.desc.Digest)
	if err != nil {
		return nil, err
	}

	li.rawManifest = b
	return li.rawManifest, nil
}

func (li *layoutImage) RawConfigFile() ([]byte, error) {
	manifest, err := li.Manifest()
	if err != nil {
		return nil, err
	}

	return Bytes(li.path, manifest.Config.Digest)
}

func (li *layoutImage) LayerByDigest(h v1.Hash) (partial.CompressedLayer, error) {
	manifest, err := li.Manifest()
	if err != nil {
		return nil, err
	}

	if h == manifest.Config.Digest {
		return partial.CompressedLayer(&compressedBlob{
			path: li.path,
			desc: manifest.Config,
		}), nil
	}

	for _, desc := range manifest.Layers {
		if h == desc.Digest {
			// We assume that all these layers are compressed, which is probably not
			// safe to assume. It will take some restructuring to make that work, so
			// just return an error for now if we encounter unexpected layers.
			if err := checkCompressedLayer(desc); err != nil {
				return nil, err
			}

			return partial.CompressedLayer(&compressedBlob{
				path: li.path,
				desc: desc,
			}), nil
		}
	}

	return nil, fmt.Errorf("could not find layer in image: %s", h)
}

func checkCompressedLayer(desc v1.Descriptor) error {
	switch desc.MediaType {
	case types.OCILayer:
	case types.DockerLayer:
	default:
		return fmt.Errorf("unexpected layer media type: %s for layer: %s", desc.MediaType, desc.Digest)
	}

	return nil
}

type compressedBlob struct {
	path string
	desc v1.Descriptor
}

func (b *compressedBlob) Digest() (v1.Hash, error) {
	return b.desc.Digest, nil
}

func (b *compressedBlob) Compressed() (io.ReadCloser, error) {
	return Blob(b.path, b.desc.Digest)
}

func (b *compressedBlob) Size() (int64, error) {
	return b.desc.Size, nil
}
