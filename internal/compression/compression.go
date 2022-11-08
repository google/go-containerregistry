// Copyright 2022 Google LLC All Rights Reserved.
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
	"bufio"
	"bytes"
	"io"

	"github.com/google/go-containerregistry/internal/gzip"
	"github.com/google/go-containerregistry/internal/zstd"
)

type Compression string

// The collection of known MediaType values.
const (
	None Compression = "none"
	GZip Compression = "gzip"
	ZStd Compression = "zstd"
)

type Opener = func() (io.ReadCloser, error)

func GetCompression(opener Opener) (Compression, error) {
	rc, err := opener()
	if err != nil {
		return None, err
	}
	defer rc.Close()

	compression, _, err := PeekCompression(rc)
	if err != nil {
		return None, err
	}

	return compression, nil
}

// PeekReader is an io.Reader that also implements Peek a la bufio.Reader.
type PeekReader interface {
	io.Reader
	Peek(n int) ([]byte, error)
}

// PeekCompression detects whether the input stream is compressed and which algorithm is used.
//
// If r implements Peek, we will use that directly, otherwise a small number
// of bytes are buffered to Peek at the gzip header, and the returned
// PeekReader can be used as a replacement for the consumed input io.Reader.
func PeekCompression(r io.Reader) (Compression, PeekReader, error) {
	var pr PeekReader
	if p, ok := r.(PeekReader); ok {
		pr = p
	} else {
		pr = bufio.NewReader(r)
	}

	var header []byte
	var err error

	if header, err = pr.Peek(2); err != nil {
		// https://github.com/google/go-containerregistry/issues/367
		if err == io.EOF {
			return None, pr, nil
		}
		return None, pr, err
	}

	if bytes.Equal(header, gzip.MagicHeader) {
		return GZip, pr, nil
	}

	if header, err = pr.Peek(4); err != nil {
		// https://github.com/google/go-containerregistry/issues/367
		if err == io.EOF {
			return None, pr, nil
		}
		return None, pr, err
	}

	if bytes.Equal(header, zstd.MagicHeader) {
		return ZStd, pr, nil
	}

	return None, pr, nil
}
