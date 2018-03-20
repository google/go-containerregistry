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

package compress

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
)

const (
	magicHeaderLength = 2
)

var (
	gzipMagicHeader = []byte{'\x1f', '\x8b'}
)

// EnsureCompressed checks if the input reader is compressed, and either passes it through or compresses it.
func EnsureCompressed(r io.Reader) (io.Reader, error) {
	r, compressed, err := IsCompressed(r)
	if err != nil {
		return nil, err
	}
	if !compressed {
		return Compress(r)
	}
	return r, nil
}

// Compress compresses the input stream.
func Compress(r io.Reader) (io.Reader, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	defer gz.Close()
	if _, err := io.Copy(gz, r); err != nil {
		return nil, err
	}
	return &buf, nil
}

// IsCompressed detects whether or not the input stream is comprssed.
func IsCompressed(r io.Reader) (io.Reader, bool, error) {
	buffered := bufio.NewReaderSize(r, magicHeaderLength)
	magic, err := buffered.Peek(magicHeaderLength)
	if err != nil {
		return nil, false, nil
	}
	if bytes.Equal(magic, gzipMagicHeader) {
		return buffered, true, nil
	}
	return buffered, false, nil
}
