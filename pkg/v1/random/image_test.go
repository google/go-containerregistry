// Copyright 2018 Google LLC All Rights Reserved.
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

package random

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"math/rand"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestManifestAndConfig(t *testing.T) {
	want := int64(12)
	img, err := Image(1024, want)
	if err != nil {
		t.Fatalf("Error loading image: %v", err)
	}
	manifest, err := img.Manifest()
	if err != nil {
		t.Fatalf("Error loading manifest: %v", err)
	}
	if got := int64(len(manifest.Layers)); got != want {
		t.Fatalf("num layers; got %v, want %v", got, want)
	}

	config, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("Error loading config file: %v", err)
	}
	if got := int64(len(config.RootFS.DiffIDs)); got != want {
		t.Fatalf("num diff ids; got %v, want %v", got, want)
	}

	if err := validate.Image(img); err != nil {
		t.Errorf("failed to validate: %v", err)
	}
}

func TestTarLayer(t *testing.T) {
	img, err := Image(1024, 5)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}
	if len(layers) != 5 {
		t.Errorf("Got %d layers, want 5", len(layers))
	}
	for i, l := range layers {
		mediaType, err := l.MediaType()
		if err != nil {
			t.Fatalf("MediaType: %v", err)
		}
		if got, want := mediaType, types.DockerLayer; got != want {
			t.Fatalf("MediaType(); got %q, want %q", got, want)
		}

		rc, err := l.Uncompressed()
		if err != nil {
			t.Errorf("Uncompressed(%d): %v", i, err)
		}
		defer rc.Close()
		tr := tar.NewReader(rc)
		if _, err := tr.Next(); err != nil {
			t.Errorf("tar.Next: %v", err)
		}

		if n, err := io.Copy(io.Discard, tr); err != nil {
			t.Errorf("Reading tar layer: %v", err)
		} else if n != 1024 {
			t.Errorf("Layer %d was %d bytes, want 1024", i, n)
		}

		if _, err := tr.Next(); !errors.Is(err, io.EOF) {
			t.Errorf("Layer contained more files; got %v, want EOF", err)
		}
	}
}

func TestRandomLayer(t *testing.T) {
	l, err := Layer(1024, types.DockerLayer)
	if err != nil {
		t.Fatalf("Layer: %v", err)
	}
	mediaType, err := l.MediaType()
	if err != nil {
		t.Fatalf("MediaType: %v", err)
	}
	if got, want := mediaType, types.DockerLayer; got != want {
		t.Errorf("MediaType(); got %q, want %q", got, want)
	}

	rc, err := l.Uncompressed()
	if err != nil {
		t.Fatalf("Uncompressed(): %v", err)
	}
	defer rc.Close()
	tr := tar.NewReader(rc)
	if _, err := tr.Next(); err != nil {
		t.Fatalf("tar.Next: %v", err)
	}

	if n, err := io.Copy(io.Discard, tr); err != nil {
		t.Errorf("Reading tar layer: %v", err)
	} else if n != 1024 {
		t.Errorf("Layer was %d bytes, want 1024", n)
	}

	if _, err := tr.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("Layer contained more files; got %v, want EOF", err)
	}
}

func TestRandomLayerSource(t *testing.T) {
	layerData := func(o ...Option) []byte {
		l, err := Layer(1024, types.DockerLayer, o...)
		if err != nil {
			t.Fatalf("Layer: %v", err)
		}

		rc, err := l.Compressed()
		if err != nil {
			t.Fatalf("Compressed(): %v", err)
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		return data
	}

	data0a := layerData(WithSource(rand.NewSource(0)))
	data0b := layerData(WithSource(rand.NewSource(0)))
	data1 := layerData(WithSource(rand.NewSource(1)))

	if !bytes.Equal(data0a, data0b) {
		t.Error("Expected the layer data to be the same with the same seed")
	}

	if bytes.Equal(data0a, data1) {
		t.Error("Expected the layer data to be different with different seeds")
	}

	dataA := layerData()
	dataB := layerData()

	if bytes.Equal(dataA, dataB) {
		t.Error("Expected the layer data to be different with different random seeds")
	}
}

func TestRandomImageSource(t *testing.T) {
	imageDigest := func(o ...Option) v1.Hash {
		img, err := Image(1024, 2, o...)
		if err != nil {
			t.Fatalf("Image: %v", err)
		}

		h, err := img.Digest()
		if err != nil {
			t.Fatalf("Digest(): %v", err)
		}
		return h
	}

	digest0a := imageDigest(WithSource(rand.NewSource(0)))
	digest0b := imageDigest(WithSource(rand.NewSource(0)))
	digest1 := imageDigest(WithSource(rand.NewSource(1)))

	if digest0a != digest0b {
		t.Error("Expected the image digest to be the same with the same seed")
	}

	if digest0a == digest1 {
		t.Error("Expected the image digest to be different with different seeds")
	}

	digestA := imageDigest()
	digestB := imageDigest()

	if digestA == digestB {
		t.Error("Expected the image digest to be different with different random seeds")
	}
}
