// Copyright 2023 Google LLC All Rights Reserved.
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

package registry_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestDiskPush(t *testing.T) {
	dir := t.TempDir()
	reg := registry.New(registry.WithBlobHandler(registry.NewDiskBlobHandler(dir)))
	srv := httptest.NewServer(reg)
	defer srv.Close()

	ref, err := name.ParseReference(strings.TrimPrefix(srv.URL, "http://") + "/foo/bar:latest")
	if err != nil {
		t.Fatal(err)
	}
	img, err := random.Image(1024, 5)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatalf("remote.Write: %v", err)
	}

	// Test we can read and validate the image.
	if _, err := remote.Image(ref); err != nil {
		t.Fatalf("remote.Image: %v", err)
	}
	if err := validate.Image(img); err != nil {
		t.Fatalf("validate.Image: %v", err)
	}

	// Collect the layer SHAs we expect to find.
	want := map[string]bool{}
	if h, err := img.ConfigName(); err != nil {
		t.Fatal(err)
	} else {
		want[fmt.Sprintf("%s/%s", h.Algorithm, h.Hex)] = true
	}
	ls, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range ls {
		if h, err := l.Digest(); err != nil {
			t.Fatal(err)
		} else {
			want[fmt.Sprintf("%s/%s", h.Algorithm, h.Hex)] = true
		}
	}

	// Test the blobs are there on disk.
	for dig := range want {
		if _, err := os.Stat(filepath.Join(dir, dig)); err != nil {
			t.Fatalf("os.Stat(%s): %v", dig, err)
		}
		t.Logf("Found %s", dig)
	}
}

func TestDiskStatCorruptBlob(t *testing.T) {
	dir := t.TempDir()
	h := writeEmptyBlobAtDigest(t, dir)

	bh := registry.NewDiskBlobHandler(dir)
	bsh, ok := bh.(registry.BlobStatHandler)
	if !ok {
		t.Fatal("NewDiskBlobHandler() does not implement Stat")
	}
	if _, err := bsh.Stat(context.Background(), "foo/bar", h); err == nil {
		t.Fatal("Stat() succeeded for corrupt blob, wanted error")
	}
}

func TestDiskCorruptBlobReturnsBlobUnknown(t *testing.T) {
	dir := t.TempDir()
	h := writeEmptyBlobAtDigest(t, dir)

	reg := registry.New(registry.WithBlobHandler(registry.NewDiskBlobHandler(dir)))
	srv := httptest.NewServer(reg)
	defer srv.Close()

	for _, method := range []string{http.MethodHead, http.MethodGet} {
		req, err := http.NewRequest(method, srv.URL+"/v2/foo/bar/blobs/"+h.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s corrupt blob status: got %d, want %d", method, resp.StatusCode, http.StatusNotFound)
		}
	}
}

func writeEmptyBlobAtDigest(t *testing.T, dir string) v1.Hash {
	t.Helper()

	h, _, err := v1.SHA256(strings.NewReader("blob contents"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, h.Algorithm), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, h.Algorithm, h.Hex), nil, 0o666); err != nil {
		t.Fatal(err)
	}
	return h
}
