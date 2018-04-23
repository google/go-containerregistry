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

package mutate

import (
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/tarball"
)

func TestNoopCondition(t *testing.T) {
	source := sourceImage(t)

	result, err := AppendLayers(source, []v1.Layer{}...)
	if err != nil {
		t.Fatalf("Unexpected error creating a writable image: %v", err)
	}

	if !manifestsAreEqual(t, source, result) {
		t.Error("manifests are not the same")
	}

	if !configFilesAreEqual(t, source, result) {
		t.Fatal("config files are not the same")
	}
}

func TestAppendWithHistory(t *testing.T) {
	source := sourceImage(t)

	addendum := Addendum{
		Layer: mockLayer{},
		History: v1.History{
			Author: "dave",
		},
	}

	result, err := Append(source, addendum)
	if err != nil {
		t.Fatalf("failed to append: %v", err)
	}

	layers := getLayers(t, result)

	if diff := cmp.Diff(layers[1], mockLayer{}); diff != "" {
		t.Fatalf("correct layer was not appended (-got, +want) %v", diff)
	}

	cf := getConfigFile(t, result)

	if diff := cmp.Diff(cf.History[1], addendum.History); diff != "" {
		t.Fatalf("the appended history is not the same (-got, +want) %s", diff)
	}
}

func TestAppendLayers(t *testing.T) {
	source := sourceImage(t)
	result, err := AppendLayers(source, mockLayer{})
	if err != nil {
		t.Fatalf("failed to append a layer: %v", err)
	}

	if manifestsAreEqual(t, source, result) {
		t.Fatal("appending a layer did not mutate the manifest")
	}

	if configFilesAreEqual(t, source, result) {
		t.Fatal("appending a layer did not mutate the config file")
	}

	layers := getLayers(t, result)

	if got, want := len(layers), 2; got != want {
		t.Fatalf("Layers did not return the appended layer "+
			"- got size %d; expected 2", len(layers))
	}

	if diff := cmp.Diff(layers[1], mockLayer{}); diff != "" {
		t.Fatalf("correct layer was not appended (-got, +want) %v", diff)
	}

	assertLayerOrderMatchesConfig(t, result)
	assertLayerOrderMatchesManifest(t, result)
	assertQueryingForLayerSucceeds(t, result, layers[1])
}

func TestMutateConfig(t *testing.T) {
	source := sourceImage(t)
	cfg, err := source.ConfigFile()
	if err != nil {
		t.Fatalf("error getting source config file")
	}

	newEnv := []string{"foo=bar"}
	cfg.Config.Env = newEnv
	result, err := Config(source, cfg.Config)
	if err != nil {
		t.Fatalf("failed to mutate a config: %v", err)
	}

	if manifestsAreEqual(t, source, result) {
		t.Fatal("mutating the config MUST mutate the manifest")
	}

	if configFilesAreEqual(t, source, result) {
		t.Fatal("mutating the config did not mutate the config file")
	}

	if !reflect.DeepEqual(cfg.Config.Env, newEnv) {
		t.Fatalf("incorrect environment set %v!=%v", cfg.Config.Env, newEnv)
	}
}

func assertQueryingForLayerSucceeds(t *testing.T, image v1.Image, layer v1.Layer) {
	t.Helper()

	queryTestCases := []struct {
		name          string
		expectedLayer v1.Layer
		hash          func() (v1.Hash, error)
		query         func(v1.Hash) (v1.Layer, error)
	}{
		{"digest", layer, layer.Digest, image.LayerByDigest},
		{"diff id", layer, layer.DiffID, image.LayerByDiffID},
	}

	for _, tc := range queryTestCases {
		t.Run(fmt.Sprintf("layer by %s", tc.name), func(t *testing.T) {
			hash, err := tc.hash()
			if err != nil {
				t.Fatalf("Unable to fetch %s for layer: %v", tc.name, err)
			}

			gotLayer, err := tc.query(hash)
			if err != nil {
				t.Fatalf("Unable to fetch layer from %s: %v", tc.name, err)
			}

			if gotLayer != tc.expectedLayer {
				t.Fatalf("Querying layer using %s does not return the expected layer %+v %+v", tc.name, gotLayer, tc.expectedLayer)
			}
		})
	}

}

func sourceImage(t *testing.T) v1.Image {
	t.Helper()

	image, err := tarball.ImageFromPath("source_image.tar", nil)
	if err != nil {
		t.Fatalf("Error loading image: %v", err)
	}
	return image
}

func getManifest(t *testing.T, i v1.Image) *v1.Manifest {
	t.Helper()

	m, err := i.Manifest()
	if err != nil {
		t.Fatalf("Error fetching image manifest: %v", err)
	}

	return m
}

func getLayers(t *testing.T, i v1.Image) []v1.Layer {
	t.Helper()

	l, err := i.Layers()
	if err != nil {
		t.Fatalf("Error fetching image layers: %v", err)
	}

	return l
}

func getConfigFile(t *testing.T, i v1.Image) *v1.ConfigFile {
	t.Helper()

	c, err := i.ConfigFile()
	if err != nil {
		t.Fatalf("Error fetching image config file: %v", err)
	}

	return c
}

func configFilesAreEqual(t *testing.T, first, second v1.Image) bool {
	t.Helper()

	fc := getConfigFile(t, first)
	sc := getConfigFile(t, second)

	return cmp.Equal(fc, sc)
}

func manifestsAreEqual(t *testing.T, first, second v1.Image) bool {
	t.Helper()

	fm := getManifest(t, first)
	sm := getManifest(t, second)

	return cmp.Equal(fm, sm)
}

func assertLayerOrderMatchesConfig(t *testing.T, i v1.Image) {
	t.Helper()

	layers := getLayers(t, i)
	cf := getConfigFile(t, i)

	if got, want := len(layers), len(cf.RootFS.DiffIDs); got != want {
		t.Fatalf("Difference in size between the image layers (%d) "+
			"and the config file diff ids (%d)", got, want)
	}

	for i := range layers {
		diffID, err := layers[i].DiffID()
		if err != nil {
			t.Fatalf("Unable to fetch layer diff id: %v", err)
		}

		if got, want := diffID, cf.RootFS.DiffIDs[i]; got != want {
			t.Fatalf("Layer diff id (%v) is not at the expected index (%d) in %+v",
				got, i, cf.RootFS.DiffIDs)
		}
	}
}

func assertLayerOrderMatchesManifest(t *testing.T, i v1.Image) {
	t.Helper()

	layers := getLayers(t, i)
	mf := getManifest(t, i)

	if got, want := len(layers), len(mf.Layers); got != want {
		t.Fatalf("Difference in size between the image layers (%d) "+
			"and the manifest layers (%d)", got, want)
	}

	for i := range layers {
		digest, err := layers[i].Digest()
		if err != nil {
			t.Fatalf("Unable to fetch layer diff id: %v", err)
		}

		if got, want := digest, mf.Layers[i].Digest; got != want {
			t.Fatalf("Layer digest (%v) is not at the expected index (%d) in %+v",
				got, i, mf.Layers)
		}
	}
}

type mockLayer struct{}

func (m mockLayer) Digest() (v1.Hash, error) {
	return v1.Hash{Algorithm: "fake", Hex: "digest"}, nil
}

func (m mockLayer) DiffID() (v1.Hash, error) {
	return v1.Hash{Algorithm: "fake", Hex: "diff id"}, nil
}

func (m mockLayer) Size() (int64, error) { return 137438691328, nil }
func (m mockLayer) Compressed() (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("compressed times")), nil
}
func (m mockLayer) Uncompressed() (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("uncompressed")), nil
}
