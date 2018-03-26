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
	"bytes"
	"io"
	"sync"

	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/types"
	"github.com/google/go-containerregistry/v1/v1util"
)

// UncompressedImageCore represents the bare minimum interface a natively
// uncompressed image must implement for us to produce a v1.Image
type UncompressedImageCore interface {
	imageCore

	// UncompressedLayer is like UncompressedBlob, but takes the "diff id".
	UncompressedLayer(v1.Hash) (io.ReadCloser, error)
}

// Assert that Image is a superset of this partial interface.
var _ UncompressedImageCore = (v1.Image)(nil)

// UncompressedToImage fills in the missing methods from an UncompressedImageCore so that it implements v1.Image.
func UncompressedToImage(uic UncompressedImageCore) (v1.Image, error) {
	return &uncompressedImageExtender{
		UncompressedImageCore: uic,
	}, nil
}

// uncompressedImageExtender implements v1.Image by extending UncompressedImageCore with the
// appropriate methods computed from the minimal core.
type uncompressedImageExtender struct {
	UncompressedImageCore

	lock     sync.Mutex
	manifest *v1.Manifest
}

// Assert that our extender type completes the v1.Image interface
var _ v1.Image = (*uncompressedImageExtender)(nil)

func (i *uncompressedImageExtender) FSLayers() ([]v1.Hash, error) {
	return FSLayers(i)
}

func (i *uncompressedImageExtender) DiffIDs() ([]v1.Hash, error) {
	return DiffIDs(i)
}

func (i *uncompressedImageExtender) BlobSet() (map[v1.Hash]struct{}, error) {
	return BlobSet(i)
}

func (i *uncompressedImageExtender) Digest() (v1.Hash, error) {
	return Digest(i)
}

func (i *uncompressedImageExtender) Manifest() (*v1.Manifest, error) {
	i.lock.Lock()
	defer i.lock.Unlock()
	if i.manifest != nil {
		return i.manifest, nil
	}

	b, err := i.RawConfigFile()
	if err != nil {
		return nil, err
	}
	cfgHash, cfgSize, err := v1.SHA256(v1util.NopReadCloser(bytes.NewBuffer(b)))
	if err != nil {
		return nil, err
	}

	m := &v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.DockerManifestSchema2,
		Config: v1.Descriptor{
			MediaType: types.DockerConfigJSON,
			Size:      cfgSize,
			Digest:    cfgHash,
		},
	}

	diffIDs, err := i.DiffIDs()
	if err != nil {
		return nil, err
	}

	for _, diffID := range diffIDs {
		rdr, err := i.Layer(diffID)
		if err != nil {
			return nil, err
		}
		h, sz, err := v1.SHA256(rdr)
		if err != nil {
			return nil, err
		}
		m.Layers = append(m.Layers, v1.Descriptor{
			MediaType: types.DockerLayer,
			Size:      sz,
			Digest:    h,
		})
	}

	i.manifest = m
	return i.manifest, nil
}

func (i *uncompressedImageExtender) RawManifest() ([]byte, error) {
	return RawManifest(i)
}

func (i *uncompressedImageExtender) ConfigName() (v1.Hash, error) {
	return ConfigName(i)
}

func (i *uncompressedImageExtender) ConfigFile() (*v1.ConfigFile, error) {
	return ConfigFile(i)
}

func (i *uncompressedImageExtender) BlobSize(h v1.Hash) (int64, error) {
	return BlobSize(i, h)
}

func (i *uncompressedImageExtender) Blob(h v1.Hash) (io.ReadCloser, error) {
	// Support returning the ConfigFile when asked for its hash.
	if cfgName, err := i.ConfigName(); err != nil {
		return nil, err
	} else if cfgName == h {
		b, err := i.RawConfigFile()
		if err != nil {
			return nil, err
		}
		return v1util.NopReadCloser(bytes.NewBuffer(b)), nil
	}

	diffID, err := BlobToDiffID(i, h)
	if err != nil {
		return nil, err
	}
	return i.Layer(diffID)
}

func (i *uncompressedImageExtender) UncompressedBlob(h v1.Hash) (io.ReadCloser, error) {
	diffID, err := BlobToDiffID(i, h)
	if err != nil {
		return nil, err
	}
	return i.UncompressedLayer(diffID)
}

func (i *uncompressedImageExtender) Layer(h v1.Hash) (io.ReadCloser, error) {
	return Layer(i, h)
}
