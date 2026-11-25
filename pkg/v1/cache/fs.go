// Copyright 2021 Google LLC All Rights Reserved.
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

package cache

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type fscache struct {
	path string
}

// NewFilesystemCache returns a Cache implementation backed by files.
func NewFilesystemCache(path string) Cache {
	return &fscache{path}
}

func (fs *fscache) Put(l v1.Layer) (v1.Layer, error) {
	digest, err := l.Digest()
	if err != nil {
		return nil, err
	}
	diffID, err := l.DiffID()
	if err != nil {
		return nil, err
	}
	return &layer{
		Layer:  l,
		path:   fs.path,
		digest: digest,
		diffID: diffID,
	}, nil
}

type layer struct {
	v1.Layer
	path           string
	digest, diffID v1.Hash
}

func (l *layer) create(h v1.Hash) (*os.File, error) {
	if err := os.MkdirAll(l.path, 0700); err != nil {
		return nil, err
	}
	return os.CreateTemp(l.path, filepath.Base(cachepath(l.path, h))+".tmp-*")
}

func (l *layer) Compressed() (io.ReadCloser, error) {
	return l.tee(l.digest, l.Layer.Compressed)
}

func (l *layer) Uncompressed() (io.ReadCloser, error) {
	return l.tee(l.diffID, l.Layer.Uncompressed)
}

func (l *layer) tee(h v1.Hash, open func() (io.ReadCloser, error)) (io.ReadCloser, error) {
	f, err := l.create(h)
	if err != nil {
		return nil, err
	}
	rc, err := open()
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	return &readcloser{rc: rc, f: f, path: cachepath(l.path, h)}, nil
}

// readcloser tees into a temporary file that Close moves to path only after
// EOF, so Get never sees a partly written layer.
type readcloser struct {
	rc   io.ReadCloser
	f    *os.File
	path string
	eof  bool
	werr error
}

func (rc *readcloser) Read(b []byte) (int, error) {
	n, err := rc.rc.Read(b)
	if n > 0 {
		if _, werr := rc.f.Write(b[:n]); werr != nil {
			rc.werr = werr
			return n, werr
		}
	}
	if errors.Is(err, io.EOF) {
		rc.eof = true
	}
	return n, err
}

func (rc *readcloser) Close() error {
	err := rc.rc.Close()
	if ferr := rc.f.Close(); err == nil {
		err = ferr
	}
	if err == nil && rc.eof && rc.werr == nil {
		if err = os.Rename(rc.f.Name(), rc.path); err == nil {
			return nil
		}
		// On Windows the rename fails while another reader has path open;
		// that copy is complete, so keep it.
		if _, serr := os.Stat(rc.path); serr == nil {
			err = nil
		}
	}
	os.Remove(rc.f.Name())
	return err
}

func (fs *fscache) Get(h v1.Hash) (v1.Layer, error) {
	l, err := tarball.LayerFromFile(cachepath(fs.path, h))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		// Delete and return ErrNotFound because the layer was incomplete.
		if err := fs.Delete(h); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	return l, err
}

func (fs *fscache) Delete(h v1.Hash) error {
	err := os.Remove(cachepath(fs.path, h))
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

func cachepath(path string, h v1.Hash) string {
	var file string
	if runtime.GOOS == "windows" {
		file = fmt.Sprintf("%s-%s", h.Algorithm, h.Hex)
	} else {
		file = h.String()
	}
	return filepath.Join(path, file)
}
