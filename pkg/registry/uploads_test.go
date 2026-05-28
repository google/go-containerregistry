// Copyright 2026 Google LLC All Rights Reserved.
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

package registry

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func TestBlobUploadSpillsToDisk(t *testing.T) {
	upload := newBlobUpload(3)
	if err := upload.Append(strings.NewReader("foo")); err != nil {
		t.Fatalf("Append() = %v", err)
	}
	if upload.file != nil {
		t.Fatal("Append() spilled before exceeding memory limit")
	}
	if err := upload.Append(strings.NewReader("bar")); err != nil {
		t.Fatalf("Append() = %v", err)
	}
	if upload.file == nil {
		t.Fatal("Append() did not spill after exceeding memory limit")
	}

	name := upload.file.Name()
	t.Cleanup(func() {
		os.Remove(name)
	})

	rc, err := upload.Reader()
	if err != nil {
		t.Fatalf("Reader() = %v", err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() = %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if string(got) != "foobar" {
		t.Fatalf("Reader() = %q, want %q", got, "foobar")
	}
	if got, want := upload.Size(), int64(6); got != want {
		t.Fatalf("Size() = %d, want %d", got, want)
	}

	if err := upload.Cleanup(); err != nil {
		t.Fatalf("Cleanup() = %v", err)
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatalf("Stat(%q) = %v, want not exist", name, err)
	}
}

func TestBlobUploadHandlerSpillsChunkToDisk(t *testing.T) {
	b := &blobs{
		blobHandler:       NewInMemoryBlobHandler(),
		uploads:           map[string]*blobUpload{},
		uploadMemoryLimit: 3,
		log:               log.New(io.Discard, "", 0),
	}

	req := httptest.NewRequest(http.MethodPatch, "/v2/foo/blobs/uploads/1", strings.NewReader("foobar"))
	resp := httptest.NewRecorder()
	if err := b.handle(resp, req); err != nil {
		t.Fatalf("PATCH handle() = %v", err)
	}
	if got, want := resp.Code, http.StatusAccepted; got != want {
		t.Fatalf("PATCH status = %d, want %d", got, want)
	}
	upload := b.uploads["1"]
	if upload == nil || upload.file == nil {
		t.Fatal("PATCH did not spill the upload to disk")
	}

	name := upload.file.Name()
	t.Cleanup(func() {
		os.Remove(name)
	})

	h, _, err := v1.SHA256(strings.NewReader("foobar"))
	if err != nil {
		t.Fatalf("SHA256() = %v", err)
	}

	req = httptest.NewRequest(http.MethodPut, "/v2/foo/blobs/uploads/1?digest="+h.String(), strings.NewReader(""))
	resp = httptest.NewRecorder()
	if err := b.handle(resp, req); err != nil {
		t.Fatalf("PUT handle() = %v", err)
	}
	if got, want := resp.Code, http.StatusCreated; got != want {
		t.Fatalf("PUT status = %d, want %d", got, want)
	}
	if _, ok := b.uploads["1"]; ok {
		t.Fatal("PUT did not remove finalized upload")
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatalf("Stat(%q) = %v, want not exist", name, err)
	}
}
