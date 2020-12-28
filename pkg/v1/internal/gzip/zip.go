// Copyright 2020 Google LLC All Rights Reserved.
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

package gzip

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"

	"github.com/google/go-containerregistry/pkg/v1/internal/and"
)

var gzipMagicHeader = []byte{'\x1f', '\x8b'}

// ReadCloser reads uncompressed input data from the io.ReadCloser and
// returns an io.ReadCloser from which compressed data may be read.
// This uses gzip.BestSpeed for the compression level.
func ReadCloser(r io.ReadCloser) io.ReadCloser {
	return ReadCloserLevel(r, gzip.BestSpeed)
}

// ReadCloserLevel reads uncompressed input data from the io.ReadCloser and
// returns an io.ReadCloser from which compressed data may be read.
// Refer to compress/gzip for the level:
// https://golang.org/pkg/compress/gzip/#pkg-constants
func ReadCloserLevel(r io.ReadCloser, level int) io.ReadCloser {
	pr, pw := io.Pipe()

	// Returns err so we can pw.CloseWithError(err)
	go func() error {
		// TODO(go1.14): Just defer {pw,gw,r}.Close like you'd expect.
		// Context: https://golang.org/issue/24283
		gw, err := gzip.NewWriterLevel(pw, level)
		if err != nil {
			return pw.CloseWithError(err)
		}

		if _, err := io.Copy(gw, r); err != nil {
			defer r.Close()
			defer gw.Close()
			return pw.CloseWithError(err)
		}
		defer pw.Close()
		defer r.Close()
		defer gw.Close()

		return nil
	}()

	return pr
}

// UnzipReadCloser reads compressed input data from the io.ReadCloser and
// returns an io.ReadCloser from which uncompessed data may be read.
func UnzipReadCloser(r io.ReadCloser) (io.ReadCloser, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &and.ReadCloser{
		Reader: gr,
		CloseFunc: func() error {
			// If the unzip fails, then this seems to return the same
			// error as the read.  We don't want this to interfere with
			// us closing the main ReadCloser, since this could leave
			// an open file descriptor (fails on Windows).
			gr.Close()
			return r.Close()
		},
	}, nil
}

// PeekReader is an io.Reader that also implements Peek a la bufio.Reader.
type PeekReader interface {
	io.Reader
	Peek(n int) ([]byte, error)
}

// Is detects whether the input stream is gzip compressed.
//
// If r implements Peek, we will use that directly, otherwise a small number
// of bytes are buffered to Peek at the gzip header, and the returned
// PeekReader can be used as a replacement for the consumed input io.Reader.
func Is(r io.Reader) (bool, PeekReader, error) {
	var pr PeekReader
	if p, ok := r.(PeekReader); ok {
		pr = p
	} else {
		pr = bufio.NewReader(r)
	}
	header, err := pr.Peek(2)
	if err != nil {
		// https://github.com/google/go-containerregistry/issues/367
		if err == io.EOF {
			return false, pr, nil
		}
		return false, pr, err
	}
	return bytes.Equal(header, gzipMagicHeader), pr, nil
}
