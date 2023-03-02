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

package compression

import (
	"bytes"
	"io"
	"testing"

	"github.com/google/go-containerregistry/internal/and"
	"github.com/google/go-containerregistry/internal/gzip"
	"github.com/google/go-containerregistry/internal/zstd"
)

type Compressor = func(rc io.ReadCloser) io.ReadCloser
type Decompressor = func(rc io.ReadCloser) (io.ReadCloser, error)

func testPeekCompression(t *testing.T,
	compressionExpected string,
	compress Compressor,
	decompress Decompressor,
) {
	content := "This is the input string."
	contentBuf := bytes.NewBufferString(content)

	compressed := compress(io.NopCloser(contentBuf))
	compressionDetected, pr, err := PeekCompression(compressed)
	if err != nil {
		t.Error("PeekCompression() =", err)
	}

	if got := string(compressionDetected); got != compressionExpected {
		t.Errorf("PeekCompression(); got %q, content %q", got, compressionExpected)
	}

	decompressed, err := decompress(withCloser(pr, compressed))
	if err != nil {
		t.Fatal(err)
	}

	b, err := io.ReadAll(decompressed)
	if err != nil {
		t.Error("ReadAll() =", err)
	}

	if got := string(b); got != content {
		t.Errorf("ReadAll(); got %q, content %q", got, content)
	}
}

func TestPeekCompression(t *testing.T) {
	testPeekCompression(t, "gzip", gzip.ReadCloser, gzip.UnzipReadCloser)
	testPeekCompression(t, "zstd", zstd.ReadCloser, zstd.UnzipReadCloser)

	nopCompress := func(rc io.ReadCloser) io.ReadCloser { return rc }
	nopDecompress := func(rc io.ReadCloser) (io.ReadCloser, error) { return rc, nil }

	testPeekCompression(t, "none", nopCompress, nopDecompress)
}

func withCloser(pr PeekReader, rc io.ReadCloser) io.ReadCloser {
	return &and.ReadCloser{
		Reader:    pr,
		CloseFunc: rc.Close,
	}
}
