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
	"bytes"
	"compress/gzip"
	"io"
)

// GzipReadCloser reads uncompressed input data from the io.ReadCloser and
// returns an io.ReadCloser from which compressed data may be read.
func GzipReadCloser(r io.ReadCloser) (io.ReadCloser, error) {
	defer r.Close()

	// TODO(mattmoor): How to avoid buffering this whole thing into memory?
	data := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(data)
	defer gw.Close()
	if _, err := io.Copy(gw, r); err != nil {
		return nil, err
	}
	return NopReadCloser(data), nil
}

// GunzipReadCloser reads compressed input data from the io.ReadCloser and
// returns an io.ReadCloser from which uncompessed data may be read.
func GunzipReadCloser(r io.ReadCloser) (io.ReadCloser, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &readAndCloser{
		Reader: gr,
		CloseFunc: func() error {
			if err := gr.Close(); err != nil {
				return err
			}
			return r.Close()
		},
	}, nil
}

// GzipWriteClose returns an io.WriteCloser to which uncompressed data may be
// written, and the compressed data is then written to the provided
// io.WriteCloser.
func GzipWriteCloser(w io.WriteCloser) io.WriteCloser {
	gw := gzip.NewWriter(w)
	return &writeAndCloser{
		Writer: gw,
		CloseFunc: func() error {
			if err := gw.Close(); err != nil {
				return err
			}
			return w.Close()
		},
	}
}

// gunzipWriteCloser implements io.WriteCloser
// It is used to implement GunzipWriteClose.
type gunzipWriteCloser struct {
	buffer *bytes.Buffer
	writer io.WriteCloser
}

// Write implements io.WriteCloser
func (gwc *gunzipWriteCloser) Write(p []byte) (n int, err error) {
	return gwc.buffer.Write(p)
}

// Close implements io.WriteCloser
func (gwc *gunzipWriteCloser) Close() error {
	// TODO(mattmoor): How to avoid buffering this whole thing into memory?
	gr, err := gzip.NewReader(gwc.buffer)
	if err != nil {
		return err
	}
	if _, err := io.Copy(gwc.writer, gr); err != nil {
		return err
	}
	return gwc.writer.Close()
}

// GunzipWriteCloser returns an io.WriteCloser to which compressed data may be
// written, and the uncompressed data is then written to the provided
// io.WriteCloser.
func GunzipWriteCloser(w io.WriteCloser) (io.WriteCloser, error) {
	return &gunzipWriteCloser{
		buffer: bytes.NewBuffer(nil),
		writer: w,
	}, nil
}
