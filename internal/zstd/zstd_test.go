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

package zstd

import (
	"bytes"
	"fmt"
	"io"
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
		{[]byte{'\x00', '\x00', '\x00', '\x00', '\x00'}, false, nil},
		{[]byte{'\x28', '\xb5', '\x2f', '\xfd', '\x1b'}, true, nil},
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
	errRead = fmt.Errorf("read failed")
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
	if r, err := UnzipReadCloser(frc); err != errRead {
		data := make([]byte, 100)
		_, err := r.Read(data)
		if err != errRead {
			t.Error("UnzipReadCloser: expected errRead, got", err)
		}
	}

	zr := ReadCloser(io.NopCloser(fr))
	if _, err := zr.Read(nil); err != errRead {
		t.Error("ReadCloser: expected errRead, got", err)
	}
}
