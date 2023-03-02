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

package tarball_test

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/internal/compare"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
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
		t.Fatalf("Error creating random image.")
	}
	tag, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag.")
	}
	if err := tarball.WriteToFile(fp.Name(), tag, randImage); err != nil {
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

	// Write the images with both tags to the tarball
	if err := tarball.MultiRefWriteToFile(fp.Name(), refToImage); err != nil {
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
		t.Fatalf("Error creating temp file.")
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())

	// Make a random image
	randImage1, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image 1.")
	}

	// Make another random image
	randImage2, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image 2.")
	}

	// Make another random image
	randImage3, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image 3.")
	}

	// Create two tags, one pointing to each image created.
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
	refToImage[tag1] = randImage1
	refToImage[tag2] = randImage2
	refToImage[dig3] = randImage3

	// Write both images to the tarball.
	if err := tarball.MultiRefWriteToFile(fp.Name(), refToImage); err != nil {
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
		t.Fatalf("Error creating temp file.")
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())

	// Make a random image
	randImage, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("Error creating random image.")
	}
	tag, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag.")
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
		t.Fatal(err)
	}
	if err := tarball.WriteToFile(fp.Name(), tag, img); err != nil {
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

func TestWriteSharedLayers(t *testing.T) {
	// Make a tempfile for tarball writes.
	fp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Error creating temp file.")
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())

	// Make a random image
	randImage, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("Error creating random image.")
	}
	tag1, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag1.")
	}
	tag2, err := name.NewTag("gcr.io/baz/bat:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag2.")
	}
	randLayer, err := random.Layer(512, types.DockerLayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}
	mutatedImage, err := mutate.Append(randImage, mutate.Addendum{
		Layer: randLayer,
	})
	if err != nil {
		t.Fatal(err)
	}
	refToImage := make(map[name.Reference]v1.Image)
	refToImage[tag1] = randImage
	refToImage[tag2] = mutatedImage

	// Write the images with both tags to the tarball
	if err := tarball.MultiRefWriteToFile(fp.Name(), refToImage); err != nil {
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

		if err := compare.Images(refToImage[tag], tarImage); err != nil {
			t.Errorf("compare.Images: %v", err)
		}
	}
	_, err = fp.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek to start of file: %v", err)
	}
	layers, err := randImage.Layers()
	if err != nil {
		t.Fatalf("Get image layers: %v", err)
	}
	layers = append(layers, randLayer)
	wantDigests := make(map[string]struct{})
	for _, layer := range layers {
		d, err := layer.Digest()
		if err != nil {
			t.Fatalf("Get layer digest: %v", err)
		}
		wantDigests[d.Hex] = struct{}{}
	}

	const layerFileSuffix = ".tar.gz"
	r := tar.NewReader(fp)
	for {
		hdr, err := r.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("Get tar header: %v", err)
		}
		if strings.HasSuffix(hdr.Name, layerFileSuffix) {
			hex := hdr.Name[:len(hdr.Name)-len(layerFileSuffix)]
			if _, ok := wantDigests[hex]; ok {
				delete(wantDigests, hex)
			} else {
				t.Errorf("Found unwanted layer with digest %q", hex)
			}
		}
	}
	if len(wantDigests) != 0 {
		for hex := range wantDigests {
			t.Errorf("Expected to find layer with digest %q but it didn't exist", hex)
		}
	}
}

func TestComputeManifest(t *testing.T) {
	var randomTag, mutatedTag = "ubuntu", "gcr.io/baz/bat:latest"

	// https://github.com/google/go-containerregistry/issues/890
	randomTagWritten := "ubuntu:latest"

	// Make a random image
	randImage, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("Error creating random image.")
	}
	randConfig, err := randImage.ConfigName()
	if err != nil {
		t.Fatalf("error getting random image config: %v", err)
	}
	tag1, err := name.NewTag(randomTag)
	if err != nil {
		t.Fatalf("Error creating test tag1.")
	}
	tag2, err := name.NewTag(mutatedTag, name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag2.")
	}
	randLayer, err := random.Layer(512, types.DockerLayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}
	mutatedImage, err := mutate.Append(randImage, mutate.Addendum{
		Layer: randLayer,
	})
	if err != nil {
		t.Fatal(err)
	}
	mutatedConfig, err := mutatedImage.ConfigName()
	if err != nil {
		t.Fatalf("error getting mutated image config: %v", err)
	}
	randomLayersHashes, err := getLayersHashes(randImage)
	if err != nil {
		t.Fatalf("error getting random image hashes: %v", err)
	}
	randomLayersFilenames := getLayersFilenames(randomLayersHashes)

	mutatedLayersHashes, err := getLayersHashes(mutatedImage)
	if err != nil {
		t.Fatalf("error getting mutated image hashes: %v", err)
	}
	mutatedLayersFilenames := getLayersFilenames(mutatedLayersHashes)

	refToImage := make(map[name.Reference]v1.Image)
	refToImage[tag1] = randImage
	refToImage[tag2] = mutatedImage

	// calculate the manifest
	m, err := tarball.ComputeManifest(refToImage)
	if err != nil {
		t.Fatalf("Unexpected error calculating manifest: %v", err)
	}
	// the order of these two is based on the repo tags
	// so mutated "gcr.io/baz/bat:latest" is before random "gcr.io/foo/bar:latest"
	expected := []tarball.Descriptor{
		{
			Config:   mutatedConfig.String(),
			RepoTags: []string{mutatedTag},
			Layers:   mutatedLayersFilenames,
		},
		{
			Config:   randConfig.String(),
			RepoTags: []string{randomTagWritten},
			Layers:   randomLayersFilenames,
		},
	}
	if len(m) != len(expected) {
		t.Fatalf("mismatched manifest lengths: actual %d, expected %d", len(m), len(expected))
	}
	mBytes, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("unable to marshall actual manifest to json: %v", err)
	}
	eBytes, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("unable to marshall expected manifest to json: %v", err)
	}
	if !bytes.Equal(mBytes, eBytes) {
		t.Errorf("mismatched manifests.\nActual: %s\nExpected: %s", string(mBytes), string(eBytes))
	}
}

func TestComputeManifest_FailsOnNoRefs(t *testing.T) {
	_, err := tarball.ComputeManifest(nil)
	if err == nil || !strings.Contains(err.Error(), "set of images is empty") {
		t.Error("expected calculateManifest to fail with nil input")
	}

	_, err = tarball.ComputeManifest(map[name.Reference]v1.Image{})
	if err == nil || !strings.Contains(err.Error(), "set of images is empty") {
		t.Error("expected calculateManifest to fail with empty input")
	}
}

func getLayersHashes(img v1.Image) ([]string, error) {
	hashes := []string{}
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("error getting image layers: %w", err)
	}
	for i, l := range layers {
		hash, err := l.Digest()
		if err != nil {
			return nil, fmt.Errorf("error getting digest for layer %d: %w", i, err)
		}
		hashes = append(hashes, hash.Hex)
	}
	return hashes, nil
}

func getLayersFilenames(hashes []string) []string {
	filenames := []string{}
	for _, h := range hashes {
		filenames = append(filenames, fmt.Sprintf("%s.tar.gz", h))
	}
	return filenames
}
