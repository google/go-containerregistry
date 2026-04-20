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
	"github.com/google/go-containerregistry/pkg/v1/mutate"
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

// TestArtifactType_Fallback tests that partial.ArtifactType falls back to
// config.mediaType when the manifest has no explicit artifactType.
func TestArtifactType_Fallback(t *testing.T) {
	for _, tc := range []struct {
		desc             string
		configMediaType  types.MediaType
		wantArtifactType string
	}{{
		desc:             "standard config mediaType",
		configMediaType:  types.DockerConfigJSON,
		wantArtifactType: string(types.DockerConfigJSON),
	}, {
		desc:             "OCI config mediaType",
		configMediaType:  types.OCIConfigJSON,
		wantArtifactType: string(types.OCIConfigJSON),
	}, {
		desc:             "custom config mediaType",
		configMediaType:  "application/vnd.custom.thing",
		wantArtifactType: "application/vnd.custom.thing",
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			img, err := random.Image(1, 1)
			if err != nil {
				t.Fatal(err)
			}
			img = mutate.ConfigMediaType(img, tc.configMediaType)

			got, err := partial.ArtifactType(img)
			if err != nil {
				t.Fatalf("partial.ArtifactType: %v", err)
			}
			if got != tc.wantArtifactType {
				t.Errorf("ArtifactType: got %q, want %q", got, tc.wantArtifactType)
			}
		})
	}
}

// TestDescriptor_ArtifactType_Fallback tests that partial.Descriptor falls
// back to config.mediaType when the manifest has no explicit artifactType.
func TestDescriptor_ArtifactType_Fallback(t *testing.T) {
	for _, tc := range []struct {
		desc             string
		configMediaType  types.MediaType
		wantArtifactType string
	}{{
		desc:             "standard config mediaType",
		configMediaType:  types.DockerConfigJSON,
		wantArtifactType: string(types.DockerConfigJSON),
	}, {
		desc:             "custom config mediaType",
		configMediaType:  "application/vnd.custom.thing",
		wantArtifactType: "application/vnd.custom.thing",
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			img, err := random.Image(1, 1)
			if err != nil {
				t.Fatal(err)
			}
			img = mutate.ConfigMediaType(img, tc.configMediaType)

			desc, err := partial.Descriptor(img)
			if err != nil {
				t.Fatalf("partial.Descriptor: %v", err)
			}
			if got := desc.ArtifactType; got != tc.wantArtifactType {
				t.Errorf("ArtifactType: got %q, want %q", got, tc.wantArtifactType)
			}
		})
	}
}

// fakeWithManifest is a test helper that implements partial.WithManifest
// with a caller-controlled v1.Manifest.
type fakeWithManifest struct {
	manifest *v1.Manifest
}

func (f fakeWithManifest) Manifest() (*v1.Manifest, error) {
	return f.manifest, nil
}

func (f fakeWithManifest) RawManifest() ([]byte, error) {
	return nil, nil
}

// TestArtifactType_ExplicitArtifactType tests that partial.ArtifactType
// returns the manifest's explicit artifactType when set, rather than
// falling back to config.mediaType.
func TestArtifactType_ExplicitArtifactType(t *testing.T) {
	fake := fakeWithManifest{
		manifest: &v1.Manifest{
			SchemaVersion: 2,
			MediaType:     types.OCIManifestSchema1,
			Config: v1.Descriptor{
				MediaType: types.OCIConfigJSON,
			},
			ArtifactType: "application/vnd.my.custom.artifact",
		},
	}
	got, err := partial.ArtifactType(fake)
	if err != nil {
		t.Fatalf("partial.ArtifactType: %v", err)
	}
	if want := "application/vnd.my.custom.artifact"; got != want {
		t.Errorf("ArtifactType: got %q, want %q", got, want)
	}
}

// TestArtifactType_NilManifest tests that partial.ArtifactType returns
// empty string when the manifest is nil.
func TestArtifactType_NilManifest(t *testing.T) {
	fake := fakeWithManifest{manifest: nil}
	got, err := partial.ArtifactType(fake)
	if err != nil {
		t.Fatalf("partial.ArtifactType: %v", err)
	}
	if got != "" {
		t.Errorf("ArtifactType: got %q, want empty", got)
	}
}
