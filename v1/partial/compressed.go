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
	"io"

	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/v1util"
)

// CompressedImageCore represents the base minimum interface a natively
// compressed image must implement for us to produce a v1.Image.
type CompressedImageCore interface {
	imageCore

	// RawManifest returns the serialized bytes of the manifest.
	RawManifest() ([]byte, error)

	// Blob returns a ReadCloser for streaming the blob's content.
	Blob(v1.Hash) (io.ReadCloser, error)
}

// Assert that Image is a superset of this partial interface.
var _ CompressedImageCore = (v1.Image)(nil)

// compressedImageExtender implements v1.Image by extending CompressedImageCore with the
// appropriate methods computed from the minimal core.
type compressedImageExtender struct {
	CompressedImageCore
}

// Assert that our extender type completes the v1.Image interface
var _ v1.Image = (*compressedImageExtender)(nil)

func (i *compressedImageExtender) BlobSet() (map[v1.Hash]struct{}, error) {
	return BlobSet(i)
}

func (i *compressedImageExtender) BlobSize(h v1.Hash) (int64, error) {
	return BlobSize(i, h)
}

func (i *compressedImageExtender) Digest() (v1.Hash, error) {
	return Digest(i)
}

func (i *compressedImageExtender) ConfigName() (v1.Hash, error) {
	return ConfigName(i)
}

func (i *compressedImageExtender) DiffIDs() ([]v1.Hash, error) {
	return DiffIDs(i)
}

func (i *compressedImageExtender) FSLayers() ([]v1.Hash, error) {
	return FSLayers(i)
}

func (i *compressedImageExtender) Layer(h v1.Hash) (io.ReadCloser, error) {
	return i.Blob(h)
}

func (i *compressedImageExtender) UncompressedBlob(h v1.Hash) (io.ReadCloser, error) {
	return UncompressedBlob(i, h)
}

func (i *compressedImageExtender) UncompressedLayer(h v1.Hash) (io.ReadCloser, error) {
	h, err := DiffIDToBlob(i, h)
	if err != nil {
		return nil, err
	}
	rc, err := i.Blob(h)
	if err != nil {
		return nil, err
	}
	return v1util.GunzipReadCloser(rc)
}

func (i *compressedImageExtender) ConfigFile() (*v1.ConfigFile, error) {
	return ConfigFile(i)
}

func (i *compressedImageExtender) Manifest() (*v1.Manifest, error) {
	return Manifest(i)
}

// CompressedToImage fills in the missing methods from a CompressedImageCore so that it implements v1.Image
func CompressedToImage(cic CompressedImageCore) (v1.Image, error) {
	return &compressedImageExtender{
		CompressedImageCore: cic,
	}, nil
}
