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
	"fmt"
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Blob returns a blob with the given hash from the Path using the strict
// default read policy.
func (l Path) Blob(h v1.Hash) (io.ReadCloser, error) {
	return l.Reader().Blob(h)
}

// Blob returns a blob with the given hash using the Reader's policy.
func (r Reader) Blob(h v1.Hash) (io.ReadCloser, error) {
	return r.openBlob(h)
}

// Bytes is a convenience function to return a blob from the Path as a byte
// slice using the strict default read policy.
func (l Path) Bytes(h v1.Hash) ([]byte, error) {
	return l.Reader().Bytes(h)
}

// Bytes is a convenience function to return a blob using the Reader's policy.
func (r Reader) Bytes(h v1.Hash) ([]byte, error) {
	f, err := r.openBlob(h)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (l Path) blobPath(h v1.Hash) string {
	return l.path("blobs", h.Algorithm, h.Hex)
}

func (l Path) openBlob(h v1.Hash) (*os.File, error) {
	return l.Reader().openBlob(h)
}

func (r Reader) openBlob(h v1.Hash) (*os.File, error) {
	p := r.path.blobPath(h)
	info, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if !r.options.allowFinalSymlinks {
			return nil, fmt.Errorf("layout blob %s is a symlink", h)
		}
		info, err = os.Stat(p)
		if err != nil {
			return nil, err
		}
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("layout blob %s is not a regular file", h)
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	closeFile := true
	defer func() {
		if closeFile {
			f.Close()
		}
	}()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf("layout blob %s is not a regular file", h)
	}
	if !os.SameFile(info, stat) {
		return nil, fmt.Errorf("layout blob %s changed while opening", h)
	}
	closeFile = false
	return f, nil
}
