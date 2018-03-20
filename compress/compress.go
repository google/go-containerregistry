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
	"bytes"
	"compress/gzip"
	"io"
)

var gzipMagicHeader = []byte{'\x1f', '\x8b'}

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

// IsCompressed detects whether the input stream is compressed.
func IsCompressed(r io.Reader) (bool, error) {
	magicHeader := make([]byte, 2)
	if _, err := r.Read(magicHeader); err != nil {
		return false, err
	}
	return bytes.Equal(magicHeader, gzipMagicHeader), nil
}
