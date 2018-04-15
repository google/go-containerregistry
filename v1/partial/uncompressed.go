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

// UncompressedLayer represents the bare minimum interface a natively
// uncompressed layer must implement for us to produce a v1.Layer
type UncompressedLayer interface {
	// DiffID returns the Hash of the uncompressed layer.
	DiffID() (v1.Hash, error)

	// Uncompressed returns an io.ReadCloser for the uncompressed layer contents.
	Uncompressed() (io.ReadCloser, error)
}

// uncompressedLayerExtender implements v1.Image using the uncompressed base properties.
type uncompressedLayerExtender struct {
	UncompressedLayer
}

// Compressed implements v1.Layer
func (ule *uncompressedLayerExtender) Compressed() (io.ReadCloser, error) {
	u, err := ule.Uncompressed()
	if err != nil {
		return nil, err
	}
	return v1util.GzipReadCloser(u)
}

// Digest implements v1.Layer
func (ule *uncompressedLayerExtender) Digest() (v1.Hash, error) {
	r, err := ule.Compressed()
	if err != nil {
		return v1.Hash{}, err
	}
	h, _, err := v1.SHA256(r)
	return h, err
}

// Size implements v1.Layer
func (ule *uncompressedLayerExtender) Size() (int64, error) {
	r, err := ule.Compressed()
	if err != nil {
		return -1, err
	}
	_, i, err := v1.SHA256(r)
	return i, err
}

// UncompressedToLayer fills in the missing methos from an UncompressedLayer so that it implements v1.Layer
func UncompressedToLayer(ul UncompressedLayer) (v1.Layer, error) {
	return &uncompressedLayerExtender{ul}, nil
}

// UncompressedImageCore represents the bare minimum interface a natively
// uncompressed image must implement for us to produce a v1.Image
type UncompressedImageCore interface {
	imageCore

	// LayerByDiffID is a variation on the v1.Image method, which returns
	// an UncompressedLayer instead.
	LayerByDiffID(v1.Hash) (UncompressedLayer, error)
}

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

	ls, err := i.Layers()
	if err != nil {
		return nil, err
	}

	for _, l := range ls {
		rdr, err := l.Compressed()
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

func (i *uncompressedImageExtender) Layers() ([]v1.Layer, error) {
	diffIDs, err := DiffIDs(i)
	if err != nil {
		return nil, err
	}
	ls := make([]v1.Layer, 0, len(diffIDs))
	for _, h := range diffIDs {
		l, err := i.LayerByDiffID(h)
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)
	}
	return ls, nil
}

func (i *uncompressedImageExtender) LayerByDiffID(diffID v1.Hash) (v1.Layer, error) {
	ul, err := i.UncompressedImageCore.LayerByDiffID(diffID)
	if err != nil {
		return nil, err
	}
	return UncompressedToLayer(ul)
}

func (i *uncompressedImageExtender) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	// Support returning the ConfigFile when asked for its hash.
	if cfgName, err := i.ConfigName(); err != nil {
		return nil, err
	} else if cfgName == h {
		return ConfigLayer(i)
	}

	diffID, err := BlobToDiffID(i, h)
	if err != nil {
		return nil, err
	}
	return i.LayerByDiffID(diffID)
}
