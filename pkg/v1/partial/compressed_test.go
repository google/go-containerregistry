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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/internal/compare"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

// Remote leverages a lot of compressed partials.
func TestRemote(t *testing.T) {
	rnd, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	s, err := registry.TLS("gcr.io")
	if err != nil {
		t.Fatal(err)
	}
	tr := s.Client().Transport

	src := "gcr.io/test/compressed"
	ref, err := name.ParseReference(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, rnd, remote.WithTransport(tr)); err != nil {
		t.Fatal(err)
	}

	img, err := remote.Image(ref, remote.WithTransport(tr))
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
