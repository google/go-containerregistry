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

func (l *layer) create(h v1.Hash) (*cacheFile, error) {
	if err := os.MkdirAll(l.path, 0700); err != nil {
		return nil, err
	}
	// Write to a temporary file in the same directory, so the layer only
	// appears under its final name once it has been written completely.
	// Writing straight to the final path would let a concurrent Get
	// observe (or delete) a partially written layer.
	f, err := os.CreateTemp(l.path, "tmp-*")
	if err != nil {
		return nil, err
	}
	return &cacheFile{File: f, final: cachepath(l.path, h)}, nil
}

// cacheFile is a layer being written to the cache. Its contents are not
// visible to Get until commit is called.
type cacheFile struct {
	*os.File
	final string
}

// commit publishes the fully written file under its final cache path.
func (f *cacheFile) commit() error {
	return os.Rename(f.Name(), f.final)
}

// abort discards the partially written file.
func (f *cacheFile) abort() error {
	return os.Remove(f.Name())
}

func (l *layer) Compressed() (io.ReadCloser, error) {
	f, err := l.create(l.digest)
	if err != nil {
		return nil, err
	}
	rc, err := l.Layer.Compressed()
	if err != nil {
		f.Close()
		f.abort()
		return nil, err
	}
	return &readcloser{
		t:      io.TeeReader(rc, f),
		f:      f,
		closes: []func() error{rc.Close, f.Close},
	}, nil
}

func (l *layer) Uncompressed() (io.ReadCloser, error) {
	f, err := l.create(l.diffID)
	if err != nil {
		return nil, err
	}
	rc, err := l.Layer.Uncompressed()
	if err != nil {
		f.Close()
		f.abort()
		return nil, err
	}
	return &readcloser{
		t:      io.TeeReader(rc, f),
		f:      f,
		closes: []func() error{rc.Close, f.Close},
	}, nil
}

type readcloser struct {
	t      io.Reader
	f      *cacheFile
	closes []func() error
	eof    bool
}

func (rc *readcloser) Read(b []byte) (int, error) {
	n, err := rc.t.Read(b)
	if errors.Is(err, io.EOF) {
		rc.eof = true
	}
	return n, err
}

func (rc *readcloser) Close() error {
	// Call all Close methods, even if any returned an error. Return the
	// first returned error.
	var err error
	for _, c := range rc.closes {
		lastErr := c()
		if err == nil {
			err = lastErr
		}
	}
	if err == nil && rc.eof {
		// The layer was consumed in full; publish it atomically.
		err = rc.f.commit()
	} else if rerr := rc.f.abort(); err == nil {
		// The layer was only partially consumed, or closing it failed;
		// discard the partial copy instead of publishing it.
		err = rerr
	}
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
