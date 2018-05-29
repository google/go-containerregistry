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

package remote

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/google/go-containerregistry/v1"
)

// ErrCacheMiss is the error returned by implementations of Cache.Load when the
// blob was not found in the cache.
var ErrCacheMiss = errors.New("blob not found in cache")

// Cache abstracts loading and storing blob data in a cache.
type Cache interface {
	Load(v1.Hash) (io.ReadCloser, error)
	Store(v1.Hash, io.Reader) error
}

// NewDiskCache returns a Cache backed by a directory on disk, rooted under the
// specified temp dir.
func NewDiskCache(tmpdir string) Cache { return diskCache{tmpdir} }

type diskCache struct{ tmpdir string }

var _ Cache = (*diskCache)(nil)

func sanitizeHash(h v1.Hash) string {
	return strings.Replace(h.String(), ":", "_", -1)
}

func (c diskCache) Load(h v1.Hash) (io.ReadCloser, error) {
	f, err := os.Open(path.Join(c.tmpdir, sanitizeHash(h)))
	if os.IsNotExist(err) {
		return nil, ErrCacheMiss
	} else if err != nil {
		return nil, err
	}
	log.Println("reading from cache", h)
	return f, nil
}

func (c diskCache) Store(h v1.Hash, rc io.Reader) error {
	f, err := os.Create(path.Join(c.tmpdir, sanitizeHash(h)))
	if err != nil {
		return err
	}
	_, err = io.Copy(f, rc)
	log.Println("wrote to cache", h)
	return err
}

// NewMemCache returns a Cache backed by data in memory.
func NewMemCache() Cache { return &memcache{map[v1.Hash][]byte{}} }

type memcache struct{ m map[v1.Hash][]byte }

var _ Cache = (*memcache)(nil)

func (m *memcache) Store(h v1.Hash, r io.Reader) error {
	all, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	m.m[h] = all
	return nil
}

func (m *memcache) Load(h v1.Hash) (io.ReadCloser, error) {
	b, ok := m.m[h]
	if !ok {
		return nil, ErrCacheMiss
	}
	return ioutil.NopCloser(bytes.NewReader(b)), nil
}

// ReadOnly returns a Cache that only reads from the underlying cache and never
// stores any new data.
//
// It can be useful in situations where the backing store does not accept
// writes, such as a read-only persistent disk containing a cache of blobs
// populated by some other process.
func ReadOnly(c Cache) Cache { return readonly{c} }

type readonly struct{ Cache }

func (readonly) Store(v1.Hash, io.Reader) error { return nil }
