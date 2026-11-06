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
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"
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

type closeTrackingReader struct {
	io.Reader
	done  chan struct{}
	count atomic.Int32
}

func (r *closeTrackingReader) Close() error {
	if r.count.Add(1) == 1 {
		close(r.done)
	}
	return nil
}

func waitForSourceClose(t *testing.T, r *closeTrackingReader) {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(time.Second):
		t.Fatal("source not closed after output consumer exited")
	}
	if n := r.count.Load(); n != 1 {
		t.Fatalf("source closed %d times, want 1", n)
	}
}

func TestReadCloserClosesSourceOnConsumerClose(t *testing.T) {
	r := &closeTrackingReader{Reader: bytes.NewReader(bytes.Repeat([]byte("synthetic layer"), 100)), done: make(chan struct{})}
	out := ReadCloser(r)
	t.Cleanup(func() { out.Close() })
	if _, err := io.ReadFull(out, make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	waitForSourceClose(t, r)
}

func TestReadCloserClosesSourceOnEOF(t *testing.T) {
	input := bytes.Repeat([]byte("synthetic layer"), 100)
	r := &closeTrackingReader{Reader: bytes.NewReader(input), done: make(chan struct{})}
	out := ReadCloser(r)
	t.Cleanup(func() { out.Close() })
	zipped, err := io.ReadAll(out)
	if err != nil {
		t.Fatal(err)
	}
	out.Close()
	waitForSourceClose(t, r)
	decoded, err := UnzipReadCloser(io.NopCloser(bytes.NewReader(zipped)))
	if err != nil {
		t.Fatal(err)
	}
	defer decoded.Close()
	got, err := io.ReadAll(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, input) {
		t.Fatal("roundtrip data changed")
	}
}

func TestReadCloserClosesSourceOnReadError(t *testing.T) {
	r := &closeTrackingReader{Reader: cleanupErrorReader{}, done: make(chan struct{})}
	out := ReadCloser(r)
	t.Cleanup(func() { out.Close() })
	if _, err := io.ReadAll(out); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("read error=%v, want original error", err)
	}
	waitForSourceClose(t, r)
}

type cleanupErrorReader struct{}

func (cleanupErrorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
