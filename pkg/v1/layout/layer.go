// Copyright 2019 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package layout

import (
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// remoteImagelayer implements partial.CompressedLayer
type localLayer struct {
	path   Path
	digest v1.Hash
}

// Compressed implements partial.CompressedLayer.
func (ll *localLayer) Compressed() (io.ReadCloser, error) {
	return ll.path.Blob(ll.digest)
}

// Digest implements partial.CompressedLayer.
func (ll *localLayer) Digest() (v1.Hash, error) {
	return ll.digest, nil
}

// MediaType implements partial.CompressedLayer.
func (ll *localLayer) MediaType() (types.MediaType, error) {
	// TODO
	return types.DockerLayer, nil
}

// Size implements partial.CompressedLayer.
func (ll *localLayer) Size() (int64, error) {
	return ll.path.BlobSize(ll.digest)
}

// See partial.Exists.
func (ll *localLayer) Exists() (bool, error) {
	return ll.path.BlobExists(ll.digest), nil
}

var _ partial.CompressedLayer = (*localLayer)(nil)
