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

package v1util

import (
	"compress/gzip"
	"io"
)

// GunzipReadCloser is a variant of gzip.NewReader that return an io.ReadCloser
// that calls the inputs Close() method.
//    compressed ==> uncompressed
func GunzipReadCloser(r io.ReadCloser) (io.ReadCloser, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &ReadAndCloser{
		Reader: gr,
		Closer: r.Close,
	}, nil
}

// TODO(mattmoor): GzipReadCloser
//    uncompressed ==> compressed

// GzipWriteClose is a variant of gzip.NewWriter that return an io.WriteCloser
// that calls the inputs Close() method.
//    uncompressed ==> ucompressed
func GzipWriteClose(w io.WriteCloser) io.WriteCloser {
	return &WriteAndCloser{
		Writer: gzip.NewWriter(w),
		Closer: w.Close,
	}
}

// TODO(mattmoor): GunzipWriteCloser
//    compressed ==> uncompressed
