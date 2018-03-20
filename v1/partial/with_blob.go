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

// withBlob defines the subset of v1.Image used by these helper methods
type withBlob interface {
	imageCore

	// Blob returns a ReadCloser for streaming the blob's content.
	Blob(v1.Hash) (io.ReadCloser, error)
}

// UncompressedBlob returns a ReadCloser for streaming the blob's content uncompressed.
func UncompressedBlob(b withBlob, h v1.Hash) (io.ReadCloser, error) {
	rc, err := b.Blob(h)
	if err != nil {
		return nil, err
	}
	return v1util.GunzipReadCloser(rc)
}
