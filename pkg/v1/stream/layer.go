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

package stream

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"

	"github.com/google/go-containerregistry/pkg/v1"
)

// ErrNotComputed is returned when the requested value is not yet computed
// because the stream has not been consumed yet.
var ErrNotComputed = errors.New("value not computed until stream is consumed")

type Layer struct {
	blob io.ReadCloser

	digest, diffID *v1.Hash
	size           int64
}

var _ v1.Layer = (*Layer)(nil)

func NewLayer(rc io.ReadCloser) *Layer { return &Layer{blob: rc} }

func (l *Layer) Digest() (v1.Hash, error) {
	if l.digest == nil {
		return v1.Hash{}, ErrNotComputed
	}
	return *l.digest, nil
}

func (l *Layer) DiffID() (v1.Hash, error) {
	if l.diffID == nil {
		return v1.Hash{}, ErrNotComputed
	}
	return *l.diffID, nil
}

func (l *Layer) Size() (int64, error) {
	if l.size == 0 {
		return 0, ErrNotComputed
	}
	return l.size, nil
}

func (l *Layer) Uncompressed() (io.ReadCloser, error) {
	return nil, errors.New("NYI: stream.Layer.Uncompressed is not implemented")
}

func (l *Layer) Compressed() (io.ReadCloser, error) { return newCompressedReader(l) }

type compressedReader struct {
	closer io.Closer // original blob's Closer.

	h, zh hash.Hash // collects digests of compressed and uncompressed stream.
	pr    io.Reader
	count *countWriter

	l *Layer // stream.Layer to update upon Close.
}

func newCompressedReader(l *Layer) (*compressedReader, error) {
	h := sha256.New()
	zh := sha256.New()
	count := &countWriter{}

	// gzip.Writer writes to the output stream, a hash calculator to
	// capture compressed digest, and a countWriter to capture compressed
	// size.
	zw, err := gzip.NewWriterLevel(io.MultiWriter(zh, count), gzip.BestSpeed)
	if err != nil {
		return nil, err
	}

	// Pipe the contents of rc through
	pr, pw := io.Pipe()
	go func() {
		_, err := io.Copy(pw, io.TeeReader(l.blob, io.MultiWriter(h, zw)))
		pw.CloseWithError(err)
	}()

	return &compressedReader{
		closer: newMultiCloser(zw, l.blob),
		pr:     pr,
		h:      h,
		zh:     zh,
		count:  count,
		l:      l,
	}, nil
}

// multiCloser is a Closer that collects multiple Closers and Closes them in order.
type multiCloser []io.Closer

var _ io.Closer = (multiCloser)(nil)

func newMultiCloser(c ...io.Closer) multiCloser { return multiCloser(c) }

func (m multiCloser) Close() error {
	for _, c := range m {
		if err := c.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (cr *compressedReader) Read(b []byte) (int, error) { return cr.pr.Read(b) }

func (cr *compressedReader) Close() error {
	// Close the inner ReadCloser.
	if err := cr.closer.Close(); err != nil {
		return err
	}

	diffID, err := v1.NewHash("sha256:" + hex.EncodeToString(cr.h.Sum(nil)))
	if err != nil {
		return err
	}
	cr.l.diffID = &diffID

	digest, err := v1.NewHash("sha256:" + hex.EncodeToString(cr.zh.Sum(nil)))
	if err != nil {
		return err
	}
	cr.l.digest = &digest

	cr.l.size = cr.count.n
	return nil
}

// countWriter counts bytes written to it.
type countWriter struct{ n int64 }

func (c *countWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}
