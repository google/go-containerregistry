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
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/v1util"
)

// WithManifest defines the subset of v1.Image used by these helper methods
type WithManifest interface {
	imageCore

	// Manifest returns this image's Manifest object.
	Manifest() (*v1.Manifest, error)
}

// FSLayers is a helper for implementing v1.Image
func FSLayers(i WithManifest) ([]v1.Hash, error) {
	m, err := i.Manifest()
	if err != nil {
		return nil, err
	}
	fsl := make([]v1.Hash, len(m.Layers))
	for i, l := range m.Layers {
		fsl[len(fsl)-i-1] = l.Digest
	}
	return fsl, nil
}

// BlobSet is a helper for implementing v1.Image
func BlobSet(i WithManifest) (map[v1.Hash]struct{}, error) {
	m, err := i.Manifest()
	if err != nil {
		return nil, err
	}
	bs := make(map[v1.Hash]struct{})
	for _, l := range m.Layers {
		bs[l.Digest] = struct{}{}
	}
	bs[m.Config.Digest] = struct{}{}
	return bs, nil
}

// BlobSize is a helper for implementing v1.Image
func BlobSize(i WithManifest, h v1.Hash) (int64, error) {
	m, err := i.Manifest()
	if err != nil {
		return -1, err
	}
	for _, l := range m.Layers {
		if l.Digest == h {
			return l.Size, nil
		}
	}
	return -1, fmt.Errorf("blob %v not found", h)
}

// Digest is a helper for implementing v1.Image
func Digest(i WithManifest) (v1.Hash, error) {
	m, err := i.Manifest()
	if err != nil {
		return v1.Hash{}, err
	}
	b := bytes.NewBuffer(nil)
	if err := json.NewEncoder(b).Encode(m); err != nil {
		return v1.Hash{}, err
	}
	h, _, err := v1.SHA256(v1util.NopReadCloser(b))
	return h, err
}

// BlobToDiffID is a helper for mapping between compressed
// and uncompressed blob hashes.
func BlobToDiffID(i WithManifest, h v1.Hash) (v1.Hash, error) {
	blobs, err := FSLayers(i)
	if err != nil {
		return v1.Hash{}, err
	}
	diffIDs, err := DiffIDs(i)
	if err != nil {
		return v1.Hash{}, err
	}
	if len(blobs) != len(diffIDs) {
		return v1.Hash{}, fmt.Errorf("mismatched fs layers (%d) and diff ids (%d)", len(blobs), len(diffIDs))
	}
	for i, blob := range blobs {
		if blob == h {
			return diffIDs[i], nil
		}
	}
	return v1.Hash{}, fmt.Errorf("unknown blob %v", h)
}

// DiffIDtoBlob is a helper for mapping between uncompressed
// and compressed blob hashes.
func DiffIDToBlob(i WithManifest, h v1.Hash) (v1.Hash, error) {
	blobs, err := FSLayers(i)
	if err != nil {
		return v1.Hash{}, err
	}
	diffIDs, err := DiffIDs(i)
	if err != nil {
		return v1.Hash{}, err
	}
	if len(blobs) != len(diffIDs) {
		return v1.Hash{}, fmt.Errorf("mismatched fs layers (%d) and diff ids (%d)", len(blobs), len(diffIDs))
	}
	for i, diffId := range diffIDs {
		if diffId == h {
			return blobs[i], nil
		}
	}
	return v1.Hash{}, fmt.Errorf("unknown diffID %v", h)

}
