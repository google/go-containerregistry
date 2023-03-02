// Copyright 2021 Google LLC All Rights Reserved.
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

package cache

import (
	"errors"
	"io"
	"os"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestFilesystemCache(t *testing.T) {
	dir := t.TempDir()

	numLayers := 5
	img, err := random.Image(10, int64(numLayers))
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	c := NewFilesystemCache(dir)
	img = Image(img, c)

	// Read all the (compressed) layers to populate the cache.
	ls, err := img.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}
	for i, l := range ls {
		rc, err := l.Compressed()
		if err != nil {
			t.Fatalf("layer[%d].Compressed: %v", i, err)
		}
		if _, err := io.Copy(io.Discard, rc); err != nil {
			t.Fatalf("Error reading contents: %v", err)
		}
		rc.Close()
	}

	// Check that layers exist in the fs cache.
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if got, want := len(dirEntries), numLayers; got != want {
		t.Errorf("Got %d cached files, want %d", got, want)
	}
	for _, de := range dirEntries {
		fi, err := de.Info()
		if err != nil {
			t.Fatal(err)
		}
		if fi.Size() == 0 {
			t.Errorf("Cached file %q is empty", fi.Name())
		}
	}

	// Read all (uncompressed) layers, those populate the cache too.
	for i, l := range ls {
		rc, err := l.Uncompressed()
		if err != nil {
			t.Fatalf("layer[%d].Compressed: %v", i, err)
		}
		if _, err := io.Copy(io.Discard, rc); err != nil {
			t.Fatalf("Error reading contents: %v", err)
		}
		rc.Close()
	}

	// Check that double the layers are present now, both compressed and
	// uncompressed.
	dirEntries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if got, want := len(dirEntries), numLayers*2; got != want {
		t.Errorf("Got %d cached files, want %d", got, want)
	}
	for _, de := range dirEntries {
		fi, err := de.Info()
		if err != nil {
			t.Fatal(err)
		}
		if fi.Size() == 0 {
			t.Errorf("Cached file %q is empty", fi.Name())
		}
	}

	// Delete a cached layer, see it disappear.
	l := ls[0]
	h, err := l.Digest()
	if err != nil {
		t.Fatalf("layer.Digest: %v", err)
	}
	if err := c.Delete(h); err != nil {
		t.Errorf("cache.Delete: %v", err)
	}
	dirEntries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if got, want := len(dirEntries), numLayers*2-1; got != want {
		t.Errorf("Got %d cached files, want %d", got, want)
	}

	// Read the image again, see the layer reappear.
	for i, l := range ls {
		rc, err := l.Compressed()
		if err != nil {
			t.Fatalf("layer[%d].Compressed: %v", i, err)
		}
		if _, err := io.Copy(io.Discard, rc); err != nil {
			t.Fatalf("Error reading contents: %v", err)
		}
		rc.Close()
	}

	// Check that layers exist in the fs cache.
	dirEntries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if got, want := len(dirEntries), numLayers*2; got != want {
		t.Errorf("Got %d cached files, want %d", got, want)
	}
	for _, de := range dirEntries {
		fi, err := de.Info()
		if err != nil {
			t.Fatal(err)
		}
		if fi.Size() == 0 {
			t.Errorf("Cached file %q is empty", fi.Name())
		}
	}
}

func TestErrNotFound(t *testing.T) {
	dir := t.TempDir()

	c := NewFilesystemCache(dir)
	h := v1.Hash{Algorithm: "fake", Hex: "not-found"}
	if _, err := c.Get(h); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(%q): %v", h, err)
	}
	if err := c.Delete(h); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete(%q): %v", h, err)
	}
}

func TestErrUnexpectedEOF(t *testing.T) {
	dir := t.TempDir()

	// create a random layer
	l, err := random.Layer(10, types.DockerLayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}
	rc, err := l.Compressed()
	if err != nil {
		t.Fatalf("layer.Compressed(): %v", err)
	}

	h, err := l.Digest()
	if err != nil {
		t.Fatalf("layer.Digest(): %v", err)
	}
	p := cachepath(dir, h)

	// Write only the first segment of the compressed layer to produce an
	// UnexpectedEOF error when reading it
	buf := make([]byte, 10)
	n, err := rc.Read(buf)
	if err != nil {
		t.Fatalf("Read(buf): %v", err)
	}
	if err := os.WriteFile(p, buf[:n], 0644); err != nil {
		t.Fatalf("os.WriteFile(%s, buf[:%d]): %v", p, n, err)
	}

	c := NewFilesystemCache(dir)

	// make sure LayerFromFile returns UnexpectedEOF
	if _, err := tarball.LayerFromFile(p); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("tarball.LayerFromFile(%s): expected %v, got %v", p, io.ErrUnexpectedEOF, err)
	}

	// Try to Get the layer
	if _, err := c.Get(h); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(%q): %v", h, err)
	}

	// If we had an UnexpectedEOF and the cache deleted the broken layer no file
	// should exist
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("os.Stat(%q): %v", p, err)
	}
}
