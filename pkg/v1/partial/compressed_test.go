// Copyright 2019 Google LLC All Rights Reserved.
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

package partial_test

import (
	"bytes"
	"errors"
	"io"
	"net/http/httptest"
	"net/url"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/compare"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

// Remote leverages a lot of compressed partials.
func TestRemote(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	rnd, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}

	src := path.Join(u.Host, "test/compressed")
	ref, err := name.ParseReference(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, rnd); err != nil {
		t.Fatal(err)
	}

	img, err := remote.Image(ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Image(img); err != nil {
		t.Fatal(err)
	}

	cf, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	m, err := img.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	layer, err := img.LayerByDiffID(cf.RootFS.DiffIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	d, err := layer.Digest()
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(d, m.Layers[0].Digest); diff != "" {
		t.Errorf("mismatched digest: %v", diff)
	}

	ok, err := partial.Exists(layer)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ok, true; got != want {
		t.Errorf("Exists() = %t != %t", got, want)
	}

	cl, err := partial.ConfigLayer(img)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cl.(*remote.MountableLayer); !ok {
		t.Errorf("ConfigLayer() expected to be MountableLayer, got %T", cl)
	}
}

type noDiffID struct {
	l v1.Layer
}

func (l *noDiffID) Digest() (v1.Hash, error) {
	return l.l.Digest()
}
func (l *noDiffID) Compressed() (io.ReadCloser, error) {
	return l.l.Compressed()
}
func (l *noDiffID) Size() (int64, error) {
	return l.l.Size()
}
func (l *noDiffID) MediaType() (types.MediaType, error) {
	return l.l.MediaType()
}
func (l *noDiffID) Descriptor() (*v1.Descriptor, error) {
	return partial.Descriptor(l.l)
}
func (l *noDiffID) UncompressedSize() (int64, error) {
	return partial.UncompressedSize(l.l)
}

func TestCompressedLayerExtender(t *testing.T) {
	rnd, err := random.Layer(1000, types.OCILayer)
	if err != nil {
		t.Fatal(err)
	}
	l, err := partial.CompressedToLayer(&noDiffID{rnd})
	if err != nil {
		t.Fatal(err)
	}

	if err := compare.Layers(rnd, l); err != nil {
		t.Fatalf("compare.Layers: %v", err)
	}
	if _, err := partial.Descriptor(l); err != nil {
		t.Fatalf("partial.Descriptor: %v", err)
	}
	if _, err := partial.UncompressedSize(l); err != nil {
		t.Fatalf("partial.UncompressedSize: %v", err)
	}
}

// errorReader fails every Read with err.
type errorReader struct {
	err error
}

func (r *errorReader) Read([]byte) (int, error) {
	return 0, r.err
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

// fakeCompressedLayer serves a fixed reader from Compressed.
type fakeCompressedLayer struct {
	rc io.ReadCloser
}

func (f *fakeCompressedLayer) Digest() (v1.Hash, error)            { return v1.Hash{}, nil }
func (f *fakeCompressedLayer) Compressed() (io.ReadCloser, error)  { return f.rc, nil }
func (f *fakeCompressedLayer) Size() (int64, error)                { return 0, nil }
func (f *fakeCompressedLayer) MediaType() (types.MediaType, error) { return types.OCILayer, nil }

// TestUncompressedClosesSourceOnError ensures Uncompressed closes the source
// reader when decompression setup fails, so callers can retry against the
// same failing input without accumulating unclosed resources.
// https://github.com/google/go-containerregistry/issues/2496
func TestUncompressedClosesSourceOnError(t *testing.T) {
	testCases := []struct {
		name    string
		content io.Reader
	}{{
		name:    "read error during compression detection",
		content: &errorReader{err: errors.New("boom")},
	}, {
		name:    "truncated gzip header",
		content: bytes.NewReader([]byte{'\x1f', '\x8b', 0x00, 0x00}),
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var closed int
			rc := &trackedReadCloser{Reader: tc.content, closed: &closed}
			l, err := partial.CompressedToLayer(&fakeCompressedLayer{rc: rc})
			if err != nil {
				t.Fatal(err)
			}

			// Act
			if _, err := l.Uncompressed(); err == nil {
				t.Fatal("Uncompressed() = nil error, want error")
			}

			// Assert
			if closed != 1 {
				t.Errorf("source reader closed %d times, want 1", closed)
			}
		})
	}
}

type compressedImage struct {
	img v1.Image
}

func (i *compressedImage) RawConfigFile() ([]byte, error) {
	return i.img.RawConfigFile()
}

func (i *compressedImage) MediaType() (types.MediaType, error) {
	return i.img.MediaType()
}

func (i *compressedImage) LayerByDigest(h v1.Hash) (partial.CompressedLayer, error) {
	return i.img.LayerByDigest(h)
}

func (i *compressedImage) RawManifest() ([]byte, error) {
	return i.img.RawManifest()
}

func (i *compressedImage) Descriptor() (*v1.Descriptor, error) {
	return partial.Descriptor(i.img)
}

func TestCompressed(t *testing.T) {
	rnd, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}

	core := &compressedImage{rnd}

	img, err := partial.CompressedToImage(core)
	if err != nil {
		t.Fatal(err)
	}

	if err := validate.Image(img); err != nil {
		t.Fatalf("validate.Image: %v", err)
	}
	if _, err := partial.Descriptor(img); err != nil {
		t.Fatalf("partial.Descriptor: %v", err)
	}
}
