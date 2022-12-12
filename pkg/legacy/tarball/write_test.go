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

package tarball

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/internal/compare"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestWrite(t *testing.T) {
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file.")
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())

	// Make a random image
	randImage, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image: %v", err)
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
	if err := Write(tag, randImage, o); err != nil {
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
		if err := compare.Images(randImage, tarImage); err != nil {
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

func TestMultiWriteSameImage(t *testing.T) {
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file.")
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())

	// Make a random image
	randImage, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image.")
	}

	// Make two tags that point to the random image above.
	tag1, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag1.")
	}
	tag2, err := name.NewTag("gcr.io/baz/bat:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag2.")
	}
	dig3, err := name.NewDigest("gcr.io/baz/baz@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test dig3.")
	}
	refToImage := make(map[name.Reference]v1.Image)
	refToImage[tag1] = randImage
	refToImage[tag2] = randImage
	refToImage[dig3] = randImage

	o, err := os.Create(fp.Name())
	if err != nil {
		t.Fatalf("Error creating %q to write image tarball: %v", fp.Name(), err)
	}
	defer o.Close()

	// Write the images with both tags to the tarball
	if err := MultiWrite(refToImage, o); err != nil {
		t.Fatalf("Unexpected error writing tarball: %v", err)
	}
	for ref := range refToImage {
		tag, ok := ref.(name.Tag)
		if !ok {
			continue
		}

		tarImage, err := tarball.ImageFromPath(fp.Name(), &tag)
		if err != nil {
			t.Fatalf("Unexpected error reading tarball: %v", err)
		}
		if err := validate.Image(tarImage); err != nil {
			t.Errorf("validate.Image: %v", err)
		}
		if err := compare.Images(randImage, tarImage); err != nil {
			t.Errorf("compare.Images: %v", err)
		}
	}
}

func TestMultiWriteDifferentImages(t *testing.T) {
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())

	// Make a random image
	randImage1, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image 1: %v", err)
	}

	// Make another random image
	randImage2, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image 2: %v", err)
	}

	// Make another random image
	randImage3, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image 3: %v", err)
	}

	// Create two tags, one pointing to each image created.
	tag1, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag1: %v", err)
	}
	tag2, err := name.NewTag("gcr.io/baz/bat:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag2: %v", err)
	}
	dig3, err := name.NewDigest("gcr.io/baz/baz@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test dig3: %v", err)
	}
	refToImage := make(map[name.Reference]v1.Image)
	refToImage[tag1] = randImage1
	refToImage[tag2] = randImage2
	refToImage[dig3] = randImage3

	o, err := os.Create(fp.Name())
	if err != nil {
		t.Fatalf("Error creating %q to write image tarball: %v", fp.Name(), err)
	}
	defer o.Close()

	// Write both images to the tarball.
	if err := MultiWrite(refToImage, o); err != nil {
		t.Fatalf("Unexpected error writing tarball: %v", err)
	}
	for ref, img := range refToImage {
		tag, ok := ref.(name.Tag)
		if !ok {
			continue
		}

		tarImage, err := tarball.ImageFromPath(fp.Name(), &tag)
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
}

func TestWriteForeignLayers(t *testing.T) {
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())

	// Make a random image
	randImage, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("Error creating random image: %v", err)
	}
	tag, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag: %v", err)
	}
	randLayer, err := random.Layer(512, types.DockerForeignLayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}
	img, err := mutate.Append(randImage, mutate.Addendum{
		Layer: randLayer,
		URLs: []string{
			"example.com",
		},
	})
	if err != nil {
		t.Fatalf("Unable to mutate image to add foreign layer: %v", err)
	}
	o, err := os.Create(fp.Name())
	if err != nil {
		t.Fatalf("Error creating %q to write image tarball: %v", fp.Name(), err)
	}
	defer o.Close()
	if err := Write(tag, img, o); err != nil {
		t.Fatalf("Unexpected error writing tarball: %v", err)
	}

	tarImage, err := tarball.ImageFromPath(fp.Name(), &tag)
	if err != nil {
		t.Fatalf("Unexpected error reading tarball: %v", err)
	}

	if err := validate.Image(tarImage); err != nil {
		t.Fatalf("validate.Image(): %v", err)
	}

	m, err := tarImage.Manifest()
	if err != nil {
		t.Fatal(err)
	}

	if got, want := m.Layers[1].MediaType, types.DockerForeignLayer; got != want {
		t.Errorf("Wrong MediaType: %s != %s", got, want)
	}
	if got, want := m.Layers[1].URLs[0], "example.com"; got != want {
		t.Errorf("Wrong URLs: %s != %s", got, want)
	}
}

func TestMultiWriteNoHistory(t *testing.T) {
	// Make a random image.
	img, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image: %v", err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("Error getting image config: %v", err)
	}
	// Blank out the layer history.
	cfg.History = nil
	tag, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag: %v", err)
	}
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())
	if err := Write(tag, img, fp); err != nil {
		t.Fatalf("Unexpected error writing tarball: %v", err)
	}
	tarImage, err := tarball.ImageFromPath(fp.Name(), &tag)
	if err != nil {
		t.Fatalf("Unexpected error reading tarball: %v", err)
	}
	if err := validate.Image(tarImage); err != nil {
		t.Fatalf("validate.Image(): %v", err)
	}
}

func TestMultiWriteHistoryEmptyLayers(t *testing.T) {
	// Build a history for 2 layers that is interspersed with empty layer
	// history.
	h := []v1.History{
		{EmptyLayer: true},
		{EmptyLayer: false},
		{EmptyLayer: true},
		{EmptyLayer: false},
		{EmptyLayer: true},
	}
	// Make a random image with the number of non-empty layers from the history
	// above.
	img, err := random.Image(256, int64(len(filterEmpty(h))))
	if err != nil {
		t.Fatalf("Error creating random image: %v", err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("Error getting image config: %v", err)
	}
	// Override the config history with our custom built history that includes
	// history for empty layers.
	cfg.History = h
	tag, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag: %v", err)
	}
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())
	if err := Write(tag, img, fp); err != nil {
		t.Fatalf("Unexpected error writing tarball: %v", err)
	}
	tarImage, err := tarball.ImageFromPath(fp.Name(), &tag)
	if err != nil {
		t.Fatalf("Unexpected error reading tarball: %v", err)
	}
	if err := validate.Image(tarImage); err != nil {
		t.Fatalf("validate.Image(): %v", err)
	}
}

func TestMultiWriteMismatchedHistory(t *testing.T) {
	// Make a random image
	img, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image: %v", err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("Error getting image config: %v", err)
	}

	// Set the history such that number of history entries != layers. This
	// should trigger an error during the image write.
	cfg.History = make([]v1.History, 1)
	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		t.Fatalf("mutate.ConfigFile() = %v", err)
	}

	tag, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag: %v", err)
	}
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())
	err = Write(tag, img, fp)
	if err == nil {
		t.Fatal("Unexpected success writing tarball, got nil, want error.")
	}
	want := "image config had layer history which did not match the number of layers"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Got unexpected error when writing image with mismatched history & layer, got %v, want substring %q", err, want)
	}
}

type fastSizeLayer struct {
	v1.Layer
	size   int64
	called bool
}

func (l *fastSizeLayer) UncompressedSize() (int64, error) {
	l.called = true
	return l.size, nil
}

func TestUncompressedSize(t *testing.T) {
	// Make a random image
	img, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image: %v", err)
	}

	rand, err := random.Layer(1000, types.DockerLayer)
	if err != nil {
		t.Fatal(err)
	}

	size, err := partial.UncompressedSize(rand)
	if err != nil {
		t.Fatal(err)
	}

	l := &fastSizeLayer{Layer: rand, size: size}

	img, err = mutate.AppendLayers(img, l)
	if err != nil {
		t.Fatal(err)
	}
	tag, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag: %v", err)
	}
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())
	if err := Write(tag, img, fp); err != nil {
		t.Fatalf("Write(): %v", err)
	}
	if !l.called {
		t.Errorf("expected UncompressedSize to be called, but it wasn't")
	}
}

// TestWriteSharedLayers tests that writing a tarball of multiple images that
// share some layers only writes those shared layers once.
func TestWriteSharedLayers(t *testing.T) {
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())

	const baseImageLayerCount = 8

	// Make a random image
	baseImage, err := random.Image(256, baseImageLayerCount)
	if err != nil {
		t.Fatalf("Error creating base image: %v", err)
	}

	// Make another random image
	randLayer, err := random.Layer(256, types.DockerLayer)
	if err != nil {
		t.Fatalf("Error creating random layer %v", err)
	}
	extendedImage, err := mutate.Append(baseImage, mutate.Addendum{
		Layer: randLayer,
	})
	if err != nil {
		t.Fatalf("Error mutating base image %v", err)
	}

	// Create two tags, one pointing to each image created.
	tag1, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag1: %v", err)
	}
	tag2, err := name.NewTag("gcr.io/baz/bat:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag2: %v", err)
	}
	refToImage := map[name.Reference]v1.Image{
		tag1: baseImage,
		tag2: extendedImage,
	}

	o, err := os.Create(fp.Name())
	if err != nil {
		t.Fatalf("Error creating %q to write image tarball: %v", fp.Name(), err)
	}
	defer o.Close()

	// Write both images to the tarball.
	if err := MultiWrite(refToImage, o); err != nil {
		t.Fatalf("Unexpected error writing tarball: %v", err)
	}
	for ref, img := range refToImage {
		tag, ok := ref.(name.Tag)
		if !ok {
			continue
		}

		tarImage, err := tarball.ImageFromPath(fp.Name(), &tag)
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

	wantIDs := make(map[string]struct{})
	ids, err := v1LayerIDs(baseImage)
	if err != nil {
		t.Fatalf("Error getting base image IDs: %v", err)
	}
	for _, id := range ids {
		wantIDs[id] = struct{}{}
	}
	ids, err = v1LayerIDs(extendedImage)
	if err != nil {
		t.Fatalf("Error getting extended image IDs: %v", err)
	}
	for _, id := range ids {
		wantIDs[id] = struct{}{}
	}

	// base + extended layer + different top base layer
	if len(wantIDs) != baseImageLayerCount+2 {
		t.Errorf("Expected to have %d unique layer IDs but have %d", baseImageLayerCount+2, len(wantIDs))
	}

	const layerFileName = "layer.tar"
	r := tar.NewReader(fp)
	for {
		hdr, err := r.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("Get tar header: %v", err)
		}
		if filepath.Base(hdr.Name) == layerFileName {
			id := filepath.Dir(hdr.Name)
			if _, ok := wantIDs[id]; ok {
				delete(wantIDs, id)
			} else {
				t.Errorf("Found unwanted layer with ID %q", id)
			}
		}
	}
	if len(wantIDs) != 0 {
		for id := range wantIDs {
			t.Errorf("Expected to find layer with ID %q but it didn't exist", id)
		}
	}
}

func v1LayerIDs(img v1.Image) ([]string, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("get layers: %w", err)
	}
	ids := make([]string, len(layers))
	parentID := ""
	for i, layer := range layers {
		var rawCfg []byte
		if i == len(layers)-1 {
			rawCfg, err = img.RawConfigFile()
			if err != nil {
				return nil, fmt.Errorf("get raw config file: %w", err)
			}
		}
		id, err := v1LayerID(layer, parentID, rawCfg)
		if err != nil {
			return nil, fmt.Errorf("get v1 layer ID: %w", err)
		}

		ids[i] = id
		parentID = id
	}
	return ids, nil
}
