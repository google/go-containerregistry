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

package estargz

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/internal/gzip"
)

func TestReader(t *testing.T) {
	want := "This is the input string."
	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)

	if err := tw.WriteHeader(&tar.Header{
		Name: "foo",
		Size: int64(len(want)),
	}); err != nil {
		t.Fatal("WriteHeader() =", err)
	}
	if _, err := tw.Write([]byte(want)); err != nil {
		t.Fatal("tw.Write() =", err)
	}
	tw.Close()

	zipped, _, err := ReadCloser(io.NopCloser(buf))
	if err != nil {
		t.Fatal("ReadCloser() =", err)
	}
	unzipped, err := gzip.UnzipReadCloser(zipped)
	if err != nil {
		t.Error("gzip.UnzipReadCloser() =", err)
	}
	defer unzipped.Close()

	found := false

	r := tar.NewReader(unzipped)
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatal("tar.Next() =", err)
		}

		if hdr.Name != "foo" {
			continue
		}
		found = true

		b, err := io.ReadAll(r)
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

	if !found {
		t.Error(`Did not find the expected file "foo"`)
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

	if _, _, err := ReadCloser(io.NopCloser(fr)); err != errRead {
		t.Error("ReadCloser: expected errRead, got", err)
	}

	buf := bytes.NewBufferString("not a tarball")
	if _, _, err := ReadCloser(io.NopCloser(buf)); !strings.Contains(err.Error(), "failed to parse tar file") {
		t.Error(`ReadCloser: expected "failed to parse tar file", got`, err)
	}
}
