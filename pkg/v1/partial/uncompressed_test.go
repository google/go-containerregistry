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
	"io"
	"os"
	"testing"

	"github.com/google/go-containerregistry/internal/compare"
	legacy "github.com/google/go-containerregistry/pkg/legacy/tarball"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

// foreignLayer implements both partial.Describable and partial.UncompressedLayer.
type foreignLayer struct {
	wrapped v1.Layer
}

func (l *foreignLayer) Digest() (v1.Hash, error) {
	return l.wrapped.Digest()
}

func (l *foreignLayer) Size() (int64, error) {
	return l.wrapped.Size()
}

func (l *foreignLayer) MediaType() (types.MediaType, error) {
	return types.DockerForeignLayer, nil
}

func (l *foreignLayer) Uncompressed() (io.ReadCloser, error) {
	return l.wrapped.Uncompressed()
}

func (l *foreignLayer) DiffID() (v1.Hash, error) {
	return l.wrapped.DiffID()
}

func (l *foreignLayer) Descriptor() (*v1.Descriptor, error) {
	r, err := l.wrapped.Compressed()
	if err != nil {
		return nil, err
	}
	h, sz, err := v1.SHA256(r)
	if err != nil {
		return nil, err
	}
	return &v1.Descriptor{
		Digest:    h,
		Size:      sz,
		MediaType: types.DockerForeignLayer,
		URLs:      []string{"http://example.com"},
	}, nil
}

func (l *foreignLayer) UncompressedSize() (int64, error) {
	return partial.UncompressedSize(l.wrapped)
}

func TestUncompressedLayer(t *testing.T) {
	randLayer, err := random.Layer(1024, types.DockerForeignLayer)
	if err != nil {
		t.Fatal(err)
	}
	l := &foreignLayer{randLayer}

	desc, err := partial.Descriptor(l)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := desc.URLs[0], "http://example.com"; want != got {
		t.Errorf("URLs[0] = %s != %s", got, want)
	}

	layer, err := partial.UncompressedToLayer(l)
	if err != nil {
		t.Fatal(err)
	}

	if err := validate.Layer(layer); err != nil {
		t.Errorf("validate.Layer: %v", err)
	}
	if _, err := partial.UncompressedSize(layer); err != nil {
		t.Errorf("partial.UncompressedSize: %v", err)
	}
}

// legacy/tarball.Write + tarball.Image leverages a lot of uncompressed partials.
//
// This is cribbed from pkg/legacy/tarball just to get intra-package coverage.
func TestLegacyWrite(t *testing.T) {
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file.")
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())

	// Make a random image + layer with Descriptor().
	randImage, err := random.Image(256, 2)
	if err != nil {
		t.Fatalf("Error creating random image: %v", err)
	}
	randLayer, err := random.Layer(1024, types.DockerForeignLayer)
	if err != nil {
		t.Fatal(err)
	}
	l, err := partial.UncompressedToLayer(&foreignLayer{randLayer})
	if err != nil {
		t.Fatal(err)
	}
	img, err := mutate.AppendLayers(randImage, l)
	if err != nil {
		t.Fatal(err)
	}
	tag, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag: %v", err)
	}
	o, err := os.Create(fp.Name())
	if err != nil {
		t.Fatalf("Error creating %q to write image tarball: %v", fp.Name(), err)
	}
	defer o.Close()
	if err := legacy.Write(tag, img, o); err != nil {
		t.Fatalf("Unexpected error writing tarball: %v", err)
	}

	// Make sure the image is valid and can be loaded.
	// Load it both by nil and by its name.
	for _, it := range []*name.Tag{nil, &tag} {
		tarImage, err := tarball.ImageFromPath(fp.Name(), it)
		if err != nil {
			t.Fatalf("Unexpected error reading tarball: %v", err)
		}
		if err := validate.Image(tarImage); err != nil {
			t.Errorf("validate.Image: %v", err)
		}
		if err := compare.Images(img, tarImage); err != nil {
			t.Errorf("compare.Images: %v", err)
		}
	}

	// Try loading a different tag, it should error.
	fakeTag, err := name.NewTag("gcr.io/notthistag:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error generating tag: %v", err)
	}
	if _, err := tarball.ImageFromPath(fp.Name(), &fakeTag); err == nil {
		t.Errorf("Expected error loading tag %v from image", fakeTag)
	}
}

type uncompressedImage struct {
	img v1.Image
}

func (i *uncompressedImage) RawConfigFile() ([]byte, error) {
	return i.img.RawConfigFile()
}

func (i *uncompressedImage) MediaType() (types.MediaType, error) {
	return i.img.MediaType()
}

func (i *uncompressedImage) LayerByDiffID(h v1.Hash) (partial.UncompressedLayer, error) {
	return i.img.LayerByDiffID(h)
}

func (i *uncompressedImage) Descriptor() (*v1.Descriptor, error) {
	return partial.Descriptor(i.img)
}

func TestUncompressed(t *testing.T) {
	rnd, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}

	core := &uncompressedImage{rnd}

	img, err := partial.UncompressedToImage(core)
	if err != nil {
		t.Fatal(err)
	}

	if err := validate.Image(img); err != nil {
		t.Fatalf("validate.Image: %v", err)
	}
	if _, err := partial.Descriptor(img); err != nil {
		t.Fatalf("partial.Descriptor: %v", err)
	}

	layers, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}
	layer, err := partial.UncompressedToLayer(&fastpathLayer{layers[0]})
	if err != nil {
		t.Fatal(err)
	}

	ok, err := partial.Exists(layer)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ok, true; got != want {
		t.Errorf("Exists() = %t != %t", got, want)
	}
}
