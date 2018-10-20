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
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/v1util"
)

type StreamableLayer struct {
	blob io.ReadCloser

	digest, diffID *v1.Hash
	size           int64
}

var _ v1.Layer = (*StreamableLayer)(nil)

func (s *StreamableLayer) Digest() (v1.Hash, error) {
	if s.digest == nil {
		return v1.Hash{}, errors.New("digest not yet computed")
	}
	return *s.digest, nil
}

func (s *StreamableLayer) DiffID() (v1.Hash, error) {
	if s.diffID == nil {
		return v1.Hash{}, errors.New("diffID not yet computed")
	}
	return *s.diffID, nil
}
func (s *StreamableLayer) Size() (int64, error) {
	if s.size == 0 {
		return 0, errors.New("size not yet computed")
	}
	return s.size, nil
}

func (s *StreamableLayer) Uncompressed() (io.ReadCloser, error) {
	return newCollectReader(s, s.blob), nil
}

func (s *StreamableLayer) Compressed() (io.ReadCloser, error) {
	u, err := s.Uncompressed()
	if err != nil {
		return nil, err
	}
	return v1util.GzipReadCloser(u)
}

// NewStreamableLayer returns a StreamableLayer backed by the ReadCloser, which
// is assumed to contain the layer's uncompressed data.
//
// If called, the streamable layer's Compressed method will return the
// compressed view of the data.
//
// Whether through Uncompressed or Compressed, closing the returned ReadCloser
// will close the underlying original ReadCloser, and will populate the layer's
// Digest, DiffID and Size; until the stream is consumed, these will return
// zero values.
func NewStreamableLayer(rc io.ReadCloser) *StreamableLayer { return &StreamableLayer{blob: rc} }

type collectReader struct {
	closer io.Closer

	tee   io.Reader
	h, zh hash.Hash
	zw    io.Closer // gzip closer
	n     int64

	// StreamableLayer to update upon Close.
	sl *StreamableLayer
}

func newCollectReader(sl *StreamableLayer, rc io.ReadCloser) io.ReadCloser {
	h := sha256.New()
	zh := sha256.New()
	zw := gzip.NewWriter(zh)

	return &collectReader{
		closer: rc,
		tee:    io.TeeReader(rc, io.MultiWriter(h, zw)),
		h:      h,
		zh:     zh,
		zw:     zw,
		sl:     sl,
	}
}

func (on *collectReader) Read(b []byte) (int, error) {
	n, err := on.tee.Read(b)
	on.n += int64(n)
	return n, err
}

func (on *collectReader) Close() error {
	// Finish flushing gzipped content to the compressed hasher.
	if err := on.zw.Close(); err != nil {
		return err
	}

	diffID, err := v1.NewHash("sha256:" + hex.EncodeToString(on.h.Sum(nil)))
	if err != nil {
		return err
	}
	on.sl.diffID = &diffID
	digest, err := v1.NewHash("sha256:" + hex.EncodeToString(on.zh.Sum(nil)))
	if err != nil {
		return err
	}
	on.sl.digest = &digest
	on.sl.size = on.n

	return on.closer.Close()
}
