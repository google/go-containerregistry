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
	"errors"
	"io"

	"github.com/google/go-containerregistry/v1"
)

// CompressedImageCore represents the base minimum interface a natively
// compressed image must implement for us to produce a v1.Image.
type CompressedImageCore interface {
	imageCore

	// Manifest returns this image's Manifest object.
	Manifest() (*v1.Manifest, error)

	// Digest returns the sha256 of this image's manifest.
	Digest() (v1.Hash, error)

	// Blob returns a ReadCloser for streaming the blob's content.
	Blob(v1.Hash) (io.ReadCloser, error)
}

// Assert that Image is a superset of this partial interface.
var _ CompressedImageCore = (v1.Image)(nil)

// CompressedToImage fills in the missing methods from a CompressedImageCore so that it implements v1.Image
func CompressedToImage(cic CompressedImageCore) (v1.Image, error) {
	return nil, errors.New("NYI: CompressedToImage")
}
