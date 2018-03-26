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

package v1

import (
	"io"

	"github.com/google/go-containerregistry/v1/types"
)

// Image defines the interface for interacting with an OCI v1 image.
type Image interface {
	// FSLayers returns the ordered collection of filesystem layers that comprise this image.
	// The order of the list is most-recent first, and oldest base layer last.
	FSLayers() ([]Hash, error)

	// DiffIDs returns the ordered list of uncompressed layer hashes (matches FSLayers).
	// The order of the list is most-recent first, and oldest base layer last.
	DiffIDs() ([]Hash, error)

	// ConfigName returns the hash of the image's config file.
	ConfigName() (Hash, error)

	// BlobSet returns an unordered collection of all the blobs in the image.
	BlobSet() (map[Hash]struct{}, error)

	// Digest returns the sha256 of this image's manifest.
	Digest() (Hash, error)

	// MediaType of this image's manifest.
	MediaType() (types.MediaType, error)

	// Manifest returns this image's Manifest object.
	Manifest() (*Manifest, error)

	// RawManifest returns the serialized bytes of Manifest()
	RawManifest() ([]byte, error)

	// ConfigFile returns this image's config file.
	ConfigFile() (*ConfigFile, error)

	// RawConfigFile returns the serialized bytes of ConfigFile()
	RawConfigFile() ([]byte, error)

	// BlobSize returns the size of the compressed blob, given its hash.
	BlobSize(Hash) (int64, error)

	// Blob returns a ReadCloser for streaming the blob's content.
	Blob(Hash) (io.ReadCloser, error)

	// Layer is the same as Blob, but takes the "diff id".
	Layer(Hash) (io.ReadCloser, error)

	// UncompressedBlob returns a ReadCloser for streaming the blob's content uncompressed.
	UncompressedBlob(Hash) (io.ReadCloser, error)

	// UncompressedLayer is like UncompressedBlob, but takes the "diff id".
	UncompressedLayer(Hash) (io.ReadCloser, error)
}
