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
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestRawConfigFile(t *testing.T) {
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}

	part, err := partial.RawConfigFile(img)
	if err != nil {
		t.Fatal(err)
	}

	method, err := img.RawConfigFile()
	if err != nil {
		t.Fatal(err)
	}

	if string(part) != string(method) {
		t.Errorf("mismatched config file: %s vs %s", part, method)
	}
}

func TestDigest(t *testing.T) {
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}

	part, err := partial.Digest(img)
	if err != nil {
		t.Fatal(err)
	}

	method, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}

	if part != method {
		t.Errorf("mismatched digest: %s vs %s", part, method)
	}
}

func TestManifest(t *testing.T) {
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}

	part, err := partial.Manifest(img)
	if err != nil {
		t.Fatal(err)
	}

	method, err := img.Manifest()
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(part, method); diff != "" {
		t.Errorf("mismatched manifest: %v", diff)
	}
}

func TestSize(t *testing.T) {
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}

	part, err := partial.Size(img)
	if err != nil {
		t.Fatal(err)
	}

	method, err := img.Size()
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(part, method); diff != "" {
		t.Errorf("mismatched size: %v", diff)
	}
}

func TestDiffIDToBlob(t *testing.T) {
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	cf, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	layers, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}
	want, err := layers[0].Digest()
	if err != nil {
		t.Fatal(err)
	}
	got, err := partial.DiffIDToBlob(img, cf.RootFS.DiffIDs[0])
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("mismatched digest: %v", diff)
	}

	if _, err := partial.DiffIDToBlob(img, want); err == nil {
		t.Errorf("expected err, got nil")
	}
}

func TestBlobToDiffID(t *testing.T) {
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	cf, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	layers, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}
	d, err := layers[0].Digest()
	if err != nil {
		t.Fatal(err)
	}
	want := cf.RootFS.DiffIDs[0]
	got, err := partial.BlobToDiffID(img, d)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("mismatched digest: %v", diff)
	}

	if _, err := partial.BlobToDiffID(img, want); err == nil {
		t.Errorf("expected err, got nil")
	}
}

func TestBlobSize(t *testing.T) {
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	m, err := img.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	want := m.Layers[0].Size
	got, err := partial.BlobSize(img, m.Layers[0].Digest)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("mismatched blob size: %v", diff)
	}

	if _, err := partial.BlobSize(img, v1.Hash{}); err == nil {
		t.Errorf("expected err, got nil")
	}
}

type fastpathLayer struct {
	v1.Layer
}

func (l *fastpathLayer) UncompressedSize() (int64, error) {
	return 100, nil
}

func (l *fastpathLayer) Exists() (bool, error) {
	return true, nil
}

func TestUncompressedSize(t *testing.T) {
	randLayer, err := random.Layer(1024, types.DockerLayer)
	if err != nil {
		t.Fatal(err)
	}
	fpl := &fastpathLayer{randLayer}
	us, err := partial.UncompressedSize(fpl)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := us, int64(100); got != want {
		t.Errorf("UncompressedSize() = %d != %d", got, want)
	}
}

func TestExists(t *testing.T) {
	randLayer, err := random.Layer(1024, types.DockerLayer)
	if err != nil {
		t.Fatal(err)
	}
	fpl := &fastpathLayer{randLayer}
	ok, err := partial.Exists(fpl)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ok, true; got != want {
		t.Errorf("Exists() = %t != %t", got, want)
	}

	ok, err = partial.Exists(randLayer)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ok, true; got != want {
		t.Errorf("Exists() = %t != %t", got, want)
	}
}
