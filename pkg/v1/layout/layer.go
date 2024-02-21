// Copyright 2019 The original author or authors
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
func (lp *localLayer) Compressed() (io.ReadCloser, error) {
	return lp.path.Blob(lp.digest)
}

// Digest implements partial.CompressedLayer.
func (lp *localLayer) Digest() (v1.Hash, error) {
	return lp.digest, nil
}

// MediaType implements partial.CompressedLayer.
func (*localLayer) MediaType() (types.MediaType, error) {
	// TODO
	return types.DockerLayer, nil
}

// Size implements partial.CompressedLayer.
func (rl *localLayer) Size() (int64, error) {
	return rl.path.BlobSize(rl.digest)
}

// See partial.Exists.
func (rl *localLayer) Exists() (bool, error) {
	return rl.path.BlobExists(rl.digest), nil
}

var _ partial.CompressedLayer = (*localLayer)(nil)
