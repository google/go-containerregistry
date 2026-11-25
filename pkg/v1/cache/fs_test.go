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
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"sync"
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

// gateReader serves up to limit bytes from the underlying reader, then
// blocks until release is closed. It simulates a slow download so tests can
// observe the cache while a layer is only partly written.
type gateReader struct {
	rc      io.ReadCloser
	limit   int64
	served  int64
	release <-chan struct{}
	open    bool
}

func (g *gateReader) Read(p []byte) (int, error) {
	if !g.open {
		if rem := g.limit - g.served; rem <= 0 {
			<-g.release
			g.open = true
		} else if int64(len(p)) > rem {
			p = p[:rem]
		}
	}
	n, err := g.rc.Read(p)
	g.served += int64(n)
	return n, err
}

func (g *gateReader) Close() error { return g.rc.Close() }

// gatedLayer wraps a layer so that Compressed returns gate readers.
type gatedLayer struct {
	v1.Layer
	wrap func(io.ReadCloser) io.ReadCloser
}

func (l gatedLayer) Compressed() (io.ReadCloser, error) {
	rc, err := l.Layer.Compressed()
	if err != nil {
		return nil, err
	}
	return l.wrap(rc), nil
}

// rawLayer is a v1.Layer whose compressed form is the raw tar itself
// (uncompressed media type), like layers shipped without compression.
type rawLayer struct {
	b []byte
	h v1.Hash
}

func (l rawLayer) Digest() (v1.Hash, error) { return l.h, nil }
func (l rawLayer) DiffID() (v1.Hash, error) { return l.h, nil }
func (l rawLayer) Size() (int64, error)     { return int64(len(l.b)), nil }
func (l rawLayer) MediaType() (types.MediaType, error) {
	return types.OCIUncompressedLayer, nil
}
func (l rawLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.b)), nil
}
func (l rawLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.b)), nil
}

func TestFilesystemCacheGetDuringWrite(t *testing.T) {
	// A gzip-compressed layer: a cache hit must be identical to the
	// original layer.
	gzipLayer, err := random.Layer(100000, types.OCILayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}
	testGetDuringWrite(t, gzipLayer, func(t *testing.T, got v1.Layer) {
		if s, err := got.Size(); err != nil {
			t.Errorf("Size: %v", err)
		} else if want, _ := gzipLayer.Size(); s != want {
			t.Errorf("Size: got %d, want %d", s, want)
		}
		if d, err := got.Digest(); err != nil {
			t.Errorf("Digest: %v", err)
		} else if want, _ := gzipLayer.Digest(); d != want {
			t.Errorf("Digest: got %s, want %s", d, want)
		}
	})

	// An uncompressed layer: its cached form is the raw tar, which
	// tarball.LayerFromFile re-presents as a compressed layer, so compare
	// the uncompressed contents instead of size and digest.
	data := make([]byte, 100000)
	for i := range data {
		data[i] = byte(i * 7)
	}
	var tb bytes.Buffer
	tw := tar.NewWriter(&tb)
	if err := tw.WriteHeader(&tar.Header{Name: "file.txt", Size: int64(len(data)), Mode: 0644}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	h, _, err := v1.SHA256(bytes.NewReader(tb.Bytes()))
	if err != nil {
		t.Fatalf("SHA256: %v", err)
	}
	testGetDuringWrite(t, rawLayer{b: tb.Bytes(), h: h}, func(t *testing.T, got v1.Layer) {
		rc, err := got.Uncompressed()
		if err != nil {
			t.Fatalf("Uncompressed: %v", err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if !bytes.Equal(b, tb.Bytes()) {
			t.Errorf("Uncompressed contents differ from the original tar (%d vs %d bytes)", len(b), tb.Len())
		}
	})
}

func testGetDuringWrite(t *testing.T, base v1.Layer, verify func(t *testing.T, got v1.Layer)) {
	t.Helper()
	dir := t.TempDir()

	digest, err := base.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	size, err := base.Size()
	if err != nil {
		t.Fatalf("Size: %v", err)
	}

	release := make(chan struct{})
	slow := gatedLayer{Layer: base, wrap: func(rc io.ReadCloser) io.ReadCloser {
		return &gateReader{rc: rc, limit: size / 2, release: release}
	}}

	c := NewFilesystemCache(dir)
	cl, err := c.Put(slow)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	rc, err := cl.Compressed()
	if err != nil {
		t.Fatalf("Compressed: %v", err)
	}

	// Consume exactly half of the layer. The rest has not been downloaded
	// yet, so the layer is only partly written to the cache at this point.
	if _, err := io.ReadFull(rc, make([]byte, size/2)); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}

	// A concurrent reader must not observe the partly written layer.
	if l, err := c.Get(digest); !errors.Is(err, ErrNotFound) {
		var got int64
		if l != nil {
			got, _ = l.Size()
		}
		t.Fatalf("Get(%s) during write: got layer (size %d of %d), err %v; want ErrNotFound", digest, got, size, err)
	}

	// Let the download finish and close the stream.
	close(release)
	if _, err := io.Copy(io.Discard, rc); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Now the complete layer must be visible.
	l, err := c.Get(digest)
	if err != nil {
		t.Fatalf("Get(%s) after write: %v", digest, err)
	}
	verify(t, l)

	// The cache directory must hold exactly the one completed layer file:
	// no temporary files may be left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if got := len(entries); got != 1 {
		names := make([]string, 0, got)
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("cache dir has %d entries %v, want exactly 1", got, names)
	}
}

func TestFilesystemCacheAbortedRead(t *testing.T) {
	dir := t.TempDir()

	base, err := random.Layer(100000, types.OCILayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}
	digest, err := base.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}

	c := NewFilesystemCache(dir)
	cl, err := c.Put(base)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	rc, err := cl.Compressed()
	if err != nil {
		t.Fatalf("Compressed: %v", err)
	}
	// Read a little, then abandon the stream without consuming it.
	if _, err := io.ReadFull(rc, make([]byte, 10)); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The incomplete layer must not be visible, and nothing may be left
	// behind in the cache directory.
	if _, err := c.Get(digest); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(%s) after aborted read: got %v, want ErrNotFound", digest, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if got := len(entries); got != 0 {
		t.Errorf("cache dir has %d entries, want 0", got)
	}
}

func TestFilesystemCacheConcurrentWriters(t *testing.T) {
	dir := t.TempDir()

	base, err := random.Layer(100000, types.OCILayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}
	digest, err := base.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	size, err := base.Size()
	if err != nil {
		t.Fatalf("Size: %v", err)
	}

	c := NewFilesystemCache(dir)

	// Two writers populate the same digest at the same time.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cl, err := c.Put(base)
			if err != nil {
				errs[i] = err
				return
			}
			rc, err := cl.Compressed()
			if err != nil {
				errs[i] = err
				return
			}
			if _, err := io.Copy(io.Discard, rc); err != nil {
				errs[i] = err
				return
			}
			errs[i] = rc.Close()
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("writer %d: %v", i, err)
		}
	}

	l, err := c.Get(digest)
	if err != nil {
		t.Fatalf("Get(%s): %v", digest, err)
	}
	if got, err := l.Size(); err != nil || got != size {
		t.Errorf("Size: got %d, %v; want %d", got, err, size)
	}
	if got, err := l.Digest(); err != nil || got != digest {
		t.Errorf("Digest: got %s, %v; want %s", got, err, digest)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if got := len(entries); got != 1 {
		t.Errorf("cache dir has %d entries, want exactly 1", got)
	}
}
