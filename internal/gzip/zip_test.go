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
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestReader(t *testing.T) {
	want := "This is the input string."
	buf := bytes.NewBufferString(want)
	zipped := ReadCloser(io.NopCloser(buf))
	unzipped, err := UnzipReadCloser(zipped)
	if err != nil {
		t.Error("UnzipReadCloser() =", err)
	}

	b, err := io.ReadAll(unzipped)
	if err != nil {
		t.Error("ReadAll() =", err)
	}
	if got := string(b); got != want {
		t.Errorf("ReadAll(); got %q, want %q", got, want)
	}
	if err := unzipped.Close(); err != nil {
		t.Error("Close() =", err)
	}
}

func TestIs(t *testing.T) {
	tests := []struct {
		in  []byte
		out bool
		err error
	}{
		{[]byte{}, false, nil},
		{[]byte{'\x00', '\x00', '\x00'}, false, nil},
		{[]byte{'\x1f', '\x8b', '\x1b'}, true, nil},
	}
	for _, test := range tests {
		reader := bytes.NewReader(test.in)
		got, err := Is(reader)
		if got != test.out {
			t.Errorf("Is; n: got %v, wanted %v\n", got, test.out)
		}
		if err != test.err {
			t.Errorf("Is; err: got %v, wanted %v\n", err, test.err)
		}
	}
}

var (
	errRead = fmt.Errorf("Read failed")
)

type failReader struct{}

func (f failReader) Read(_ []byte) (int, error) {
	return 0, errRead
}

func TestReadErrors(t *testing.T) {
	fr := failReader{}
	if _, err := Is(fr); err != errRead {
		t.Error("Is: expected errRead, got", err)
	}

	frc := io.NopCloser(fr)
	if _, err := UnzipReadCloser(frc); err != errRead {
		t.Error("UnzipReadCloser: expected errRead, got", err)
	}

	zr := ReadCloser(io.NopCloser(fr))
	if _, err := zr.Read(nil); err != errRead {
		t.Error("ReadCloser: expected errRead, got", err)
	}

	zr = ReadCloserLevel(io.NopCloser(strings.NewReader("zip me")), -10)
	if _, err := zr.Read(nil); err == nil {
		t.Error("Expected invalid level error, got:", err)
	}
}

// trackedReadCloser counts how many times Close is called.
type trackedReadCloser struct {
	io.Reader
	closed *int
}

func (t *trackedReadCloser) Close() error {
	*t.closed++
	return nil
}

// TestUnzipReadCloserClosesInputOnError ensures UnzipReadCloser closes the
// input reader when the gzip header cannot be read, so callers do not leak
// the source.
// https://github.com/google/go-containerregistry/issues/2496
func TestUnzipReadCloserClosesInputOnError(t *testing.T) {
	// Arrange
	var closed int
	rc := &trackedReadCloser{
		Reader: bytes.NewReader([]byte{'\x1f', '\x8b', 0x00, 0x00}),
		closed: &closed,
	}

	// Act
	if _, err := UnzipReadCloser(rc); err == nil {
		t.Fatal("UnzipReadCloser() = nil error, want error")
	}

	// Assert
	if closed != 1 {
		t.Errorf("input reader closed %d times, want 1", closed)
	}
}
