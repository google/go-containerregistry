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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1"
)

// Blob returns a blob with the given hash from an OCI Image Layout.
func Blob(path string, h v1.Hash) (io.ReadCloser, error) {
	return os.Open(blobPath(path, h))
}

// Bytes is a convenience function to return a blob from an OCI Image Layout as
// a byte slice.
func Bytes(path string, h v1.Hash) ([]byte, error) {
	return ioutil.ReadFile(blobPath(path, h))
}

func blobPath(path string, h v1.Hash) string {
	return filepath.Join(path, "blobs", h.Algorithm, h.Hex)
}
