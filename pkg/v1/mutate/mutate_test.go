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

package mutate_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-containerregistry/internal/verify"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestExtractWhiteout(t *testing.T) {
	img, err := tarball.ImageFromPath("testdata/whiteout_image.tar", nil)
	if err != nil {
		t.Errorf("Error loading image: %v", err)
	}
	tarPath, _ := filepath.Abs("img.tar")
	defer os.Remove(tarPath)
	tr := tar.NewReader(mutate.Extract(img))
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		name := header.Name
		for _, part := range filepath.SplitList(name) {
			if part == "foo" {
				t.Errorf("whiteout file found in tar: %v", name)
			}
		}
	}
}

func TestExtractWhiteoutDir(t *testing.T) {
	img, err := tarball.ImageFromPath("testdata/whiteout_dir.tar", nil)
	if err != nil {
		t.Errorf("Error loading image: %v", err)
	}
	tarPath, _ := filepath.Abs("img.tar")
	defer os.Remove(tarPath)
	tr := tar.NewReader(mutate.Extract(img))
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		name := header.Name
		if filepath.Base(name) == "foo" {
			t.Errorf("whiteout file found in tar: %v", name)
		}
	}
}

func TestExtractOverwrittenFile(t *testing.T) {
	img, err := tarball.ImageFromPath("testdata/overwritten_file.tar", nil)
	if err != nil {
		t.Fatalf("Error loading image: %v", err)
	}
	tr := tar.NewReader(mutate.Extract(img))
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		name := header.Name
		if strings.Contains(name, "foo.txt") {
			var buf bytes.Buffer
			buf.ReadFrom(tr)
			if strings.Contains(buf.String(), "foo") {
				t.Errorf("Contents of file were not correctly overwritten")
			}
		}
	}
}

func TestExtractClosesLayerBeforeOpeningNext(t *testing.T) {
	tokens := make(chan struct{}, 4)
	layers := make([]v1.Layer, cap(tokens)+1)
	for i := range layers {
		layers[i] = tokenLimitedLayer{tokens: tokens}
	}

	rc := mutate.Extract(tokenLimitedImage{layers: layers})
	defer rc.Close()
	if _, err := io.Copy(io.Discard, rc); err != nil {
		t.Fatalf("mutate.Extract() = %v", err)
	}
	if got := len(tokens); got != 0 {
		t.Fatalf("open layer readers after extraction: got %d, want 0", got)
	}
}

// TestExtractError tests that if there are any errors encountered
func TestExtractError(t *testing.T) {
	rc := mutate.Extract(invalidImage{})
	if _, err := io.Copy(io.Discard, rc); err == nil {
		t.Errorf("rc.Read; got nil error")
	} else if !strings.Contains(err.Error(), errInvalidImage.Error()) {
		t.Errorf("rc.Read; got %v, want %v", err, errInvalidImage)
	}
}

// TestExtractPartialRead tests that the reader can be partially read (e.g.,
// tar headers) and closed without error.
func TestExtractPartialRead(t *testing.T) {
	rc := mutate.Extract(invalidImage{})
	if _, err := io.Copy(io.Discard, io.LimitReader(rc, 1)); err != nil {
		t.Errorf("Could not read one byte from reader")
	}
	if err := rc.Close(); err != nil {
		t.Errorf("rc.Close: %v", err)
	}
}

// TestExtractRoundTrip builds an image layer containing every common filesystem
// object type (regular files, directories, symlinks, hard links, various
// permission modes) and verifies that mutate.Extract preserves all entries.
//
// This prevents regressions like #2244 where a security fix inadvertently
// dropped legitimate symlinks during extraction.
func TestExtractRoundTrip(t *testing.T) {
	// Build a tar layer with diverse filesystem objects.
	type entry struct {
		header *tar.Header
		body   string // only for regular files
	}
	entries := []entry{
		// Directories
		{header: &tar.Header{Name: "app/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: &tar.Header{Name: "app/bin/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: &tar.Header{Name: "app/lib/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: &tar.Header{Name: "app/node_modules/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: &tar.Header{Name: "app/node_modules/.bin/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: &tar.Header{Name: "restricted/", Typeflag: tar.TypeDir, Mode: 0o700}},

		// Regular files with various permissions
		{header: &tar.Header{Name: "app/main.js", Typeflag: tar.TypeReg, Mode: 0o644}, body: "console.log('hello')\n"},
		{header: &tar.Header{Name: "app/bin/run.sh", Typeflag: tar.TypeReg, Mode: 0o755}, body: "#!/bin/sh\n\n"},
		{header: &tar.Header{Name: "restricted/secret.key", Typeflag: tar.TypeReg, Mode: 0o600}, body: "secret\n"},
		{header: &tar.Header{Name: "app/lib/utils.js", Typeflag: tar.TypeReg, Mode: 0o644}, body: "// ok\n"},

		// Relative symlinks (the most common case, e.g. node_modules/.bin)
		{header: &tar.Header{Name: "app/node_modules/.bin/acorn", Typeflag: tar.TypeSymlink, Linkname: "../acorn/bin/acorn"}},
		{header: &tar.Header{Name: "app/node_modules/.bin/eslint", Typeflag: tar.TypeSymlink, Linkname: "../eslint/bin/eslint.js"}},

		// Absolute symlink (pointing within the image)
		{header: &tar.Header{Name: "app/link-to-main", Typeflag: tar.TypeSymlink, Linkname: "/app/main.js"}},

		// Symlink to a directory
		{header: &tar.Header{Name: "app/modules", Typeflag: tar.TypeSymlink, Linkname: "node_modules"}},

		// Hard link
		{header: &tar.Header{Name: "app/main-hardlink.js", Typeflag: tar.TypeLink, Linkname: "app/main.js"}},
	}

	// Write entries into a tar archive.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		if e.body != "" {
			e.header.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(e.header); err != nil {
			t.Fatalf("writing header for %s: %v", e.header.Name, err)
		}
		if e.body != "" {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatalf("writing body for %s: %v", e.header.Name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("closing tar writer: %v", err)
	}

	// Build a single-layer image from the tar.
	tarBytes := buf.Bytes()
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBytes)), nil
	})
	if err != nil {
		t.Fatalf("creating layer: %v", err)
	}
	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		t.Fatalf("appending layer: %v", err)
	}

	// Extract and collect all entries from the flattened output.
	extracted := map[string]*tar.Header{}
	extractedBodies := map[string]string{}
	tr := tar.NewReader(mutate.Extract(img))
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("reading extracted tar: %v", err)
		}
		extracted[hdr.Name] = hdr
		if hdr.Typeflag == tar.TypeReg && hdr.Size > 0 {
			var b bytes.Buffer
			if _, err := io.Copy(&b, tr); err != nil {
				t.Fatalf("reading body of %s: %v", hdr.Name, err)
			}
			extractedBodies[hdr.Name] = b.String()
		}
	}

	// Verify every input entry is present in the output with correct metadata.
	for _, e := range entries {
		name := filepath.Clean(e.header.Name)
		got, ok := extracted[name]
		if !ok {
			t.Errorf("entry %q not found in extracted output", name)
			continue
		}

		if got.Typeflag != e.header.Typeflag {
			t.Errorf("%s: typeflag = %d, want %d", name, got.Typeflag, e.header.Typeflag)
		}

		// Verify symlink and hard link targets are preserved.
		if e.header.Typeflag == tar.TypeSymlink || e.header.Typeflag == tar.TypeLink {
			if got.Linkname != e.header.Linkname {
				t.Errorf("%s: linkname = %q, want %q", name, got.Linkname, e.header.Linkname)
			}
		}

		// Verify file permissions.
		if e.header.Typeflag == tar.TypeReg || e.header.Typeflag == tar.TypeDir {
			if got.Mode != e.header.Mode {
				t.Errorf("%s: mode = %o, want %o", name, got.Mode, e.header.Mode)
			}
		}

		// Verify file contents.
		if e.body != "" {
			if extractedBodies[name] != e.body {
				t.Errorf("%s: body = %q, want %q", name, extractedBodies[name], e.body)
			}
		}
	}
}

// invalidImage is an image which returns an error when Layers() is called.
type invalidImage struct {
	v1.Image
}

var errInvalidImage = errors.New("invalid image")

func (invalidImage) Layers() ([]v1.Layer, error) {
	return nil, errInvalidImage
}

func TestNoopCondition(t *testing.T) {
	source := sourceImage(t)

	result, err := mutate.AppendLayers(source, []v1.Layer{}...)
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

func TestAppendWithAddendum(t *testing.T) {
	source := sourceImage(t)

	addendum := mutate.Addendum{
		Layer: mockLayer{},
		History: v1.History{
			Author: "dave",
		},
		URLs: []string{
			"example.com",
		},
		Annotations: map[string]string{
			"foo": "bar",
		},
		MediaType: types.MediaType("foo"),
	}

	result, err := mutate.Append(source, addendum)
	if err != nil {
		t.Fatalf("failed to append: %v", err)
	}

	layers := getLayers(t, result)

	if diff := cmp.Diff(layers[1], mockLayer{}); diff != "" {
		t.Fatalf("correct layer was not appended (-got, +want) %v", diff)
	}

	if configSizesAreEqual(t, source, result) {
		t.Fatal("adding a layer MUST change the config file size")
	}

	cf := getConfigFile(t, result)

	if diff := cmp.Diff(cf.History[1], addendum.History); diff != "" {
		t.Fatalf("the appended history is not the same (-got, +want) %s", diff)
	}

	m, err := result.Manifest()
	if err != nil {
		t.Fatalf("failed to get manifest: %v", err)
	}

	if diff := cmp.Diff(m.Layers[1].URLs, addendum.URLs); diff != "" {
		t.Fatalf("the appended URLs is not the same (-got, +want) %s", diff)
	}

	if diff := cmp.Diff(m.Layers[1].Annotations, addendum.Annotations); diff != "" {
		t.Fatalf("the appended Annotations is not the same (-got, +want) %s", diff)
	}
	if diff := cmp.Diff(m.Layers[1].MediaType, addendum.MediaType); diff != "" {
		t.Fatalf("the appended MediaType is not the same (-got, +want) %s", diff)
	}
}

func TestAppendLayers(t *testing.T) {
	source := sourceImage(t)
	layer, err := random.Layer(100, types.DockerLayer)
	if err != nil {
		t.Fatal(err)
	}
	result, err := mutate.AppendLayers(source, layer)
	if err != nil {
		t.Fatalf("failed to append a layer: %v", err)
	}

	if manifestsAreEqual(t, source, result) {
		t.Fatal("appending a layer did not mutate the manifest")
	}

	if configFilesAreEqual(t, source, result) {
		t.Fatal("appending a layer did not mutate the config file")
	}

	if configSizesAreEqual(t, source, result) {
		t.Fatal("adding a layer MUST change the config file size")
	}

	layers := getLayers(t, result)

	if got, want := len(layers), 2; got != want {
		t.Fatalf("Layers did not return the appended layer "+
			"- got size %d; expected 2", len(layers))
	}

	if layers[1] != layer {
		t.Errorf("correct layer was not appended: got %v; want %v", layers[1], layer)
	}

	if err := validate.Image(result); err != nil {
		t.Errorf("validate.Image() = %v", err)
	}
}

func TestMutateConfig(t *testing.T) {
	source := sourceImage(t)
	cfg, err := source.ConfigFile()
	if err != nil {
		t.Fatalf("error getting source config file")
	}

	newEnv := []string{"foo=bar"}
	cfg.Config.Env = newEnv
	result, err := mutate.Config(source, cfg.Config)
	if err != nil {
		t.Fatalf("failed to mutate a config: %v", err)
	}

	if manifestsAreEqual(t, source, result) {
		t.Error("mutating the config MUST mutate the manifest")
	}

	if configFilesAreEqual(t, source, result) {
		t.Error("mutating the config did not mutate the config file")
	}

	if configSizesAreEqual(t, source, result) {
		t.Error("adding an environment variable MUST change the config file size")
	}

	if configDigestsAreEqual(t, source, result) {
		t.Errorf("mutating the config MUST mutate the config digest")
	}

	if !reflect.DeepEqual(cfg.Config.Env, newEnv) {
		t.Errorf("incorrect environment set %v!=%v", cfg.Config.Env, newEnv)
	}

	if err := validate.Image(result); err != nil {
		t.Errorf("validate.Image() = %v", err)
	}
}

type arbitrary struct {
}

func (arbitrary) RawManifest() ([]byte, error) {
	return []byte(`{"hello":"world"}`), nil
}
func TestAnnotations(t *testing.T) {
	anns := map[string]string{
		"foo": "bar",
	}

	for _, c := range []struct {
		desc string
		in   partial.WithRawManifest
		want string
	}{{
		desc: "image",
		in:   empty.Image,
		want: `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","size":115,"digest":"sha256:5b943e2b943f6c81dbbd4e2eca5121f4fcc39139e3d1219d6d89bd925b77d9fe"},"layers":[],"annotations":{"foo":"bar"}}`,
	}, {
		desc: "index",
		in:   empty.Index,
		want: `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[],"annotations":{"foo":"bar"}}`,
	}, {
		desc: "arbitrary",
		in:   arbitrary{},
		want: `{"annotations":{"foo":"bar"},"hello":"world"}`,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got, err := mutate.Annotations(c.in, anns).RawManifest()
			if err != nil {
				t.Fatalf("Annotations: %v", err)
			}
			if d := cmp.Diff(c.want, string(got)); d != "" {
				t.Errorf("Diff(-want,+got): %s", d)
			}
		})
	}
}

func TestMutateCreatedAt(t *testing.T) {
	source := sourceImage(t)
	want := time.Now().Add(-2 * time.Minute)
	result, err := mutate.CreatedAt(source, v1.Time{Time: want})
	if err != nil {
		t.Fatalf("CreatedAt: %v", err)
	}

	if configDigestsAreEqual(t, source, result) {
		t.Errorf("mutating the created time MUST mutate the config digest")
	}

	got := getConfigFile(t, result).Created.Time
	if got != want {
		t.Errorf("mutating the created time MUST mutate the time from %v to %v", got, want)
	}
}

func TestMutateTime(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source v1.Image
	}{
		{
			name:   "image with matching history and layers",
			source: sourceImage(t),
		},
		{
			name:   "image with empty_layer history entries",
			source: sourceImagePath(t, "testdata/source_image_with_empty_layer_history.tar"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := time.Time{}
			result, err := mutate.Time(tc.source, want)
			if err != nil {
				t.Fatalf("failed to mutate a config: %v", err)
			}

			if configDigestsAreEqual(t, tc.source, result) {
				t.Fatal("mutating the created time MUST mutate the config digest")
			}

			mutatedOriginalConfig := getConfigFile(t, tc.source).DeepCopy()
			gotConfig := getConfigFile(t, result)

			// manually change the fields we expect to be changed by mutate.Time
			mutatedOriginalConfig.Author = ""
			mutatedOriginalConfig.Created = v1.Time{Time: want}
			for i := range mutatedOriginalConfig.History {
				mutatedOriginalConfig.History[i].Created = v1.Time{Time: want}
				mutatedOriginalConfig.History[i].Author = ""
			}

			if diff := cmp.Diff(mutatedOriginalConfig, gotConfig,
				cmpopts.IgnoreFields(v1.RootFS{}, "DiffIDs"),
			); diff != "" {
				t.Errorf("configFile() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMutateMediaType(t *testing.T) {
	want := types.OCIManifestSchema1
	wantCfg := types.OCIConfigJSON
	img := mutate.MediaType(empty.Image, want)
	img = mutate.ConfigMediaType(img, wantCfg)
	got, err := img.MediaType()
	if err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Errorf("%q != %q", want, got)
	}
	manifest, err := img.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.MediaType == "" {
		t.Error("MediaType should be set for OCI media types")
	}
	if gotCfg := manifest.Config.MediaType; gotCfg != wantCfg {
		t.Errorf("manifest.Config.MediaType = %v, wanted %v", gotCfg, wantCfg)
	}

	want = types.DockerManifestSchema2
	wantCfg = types.DockerConfigJSON
	img = mutate.MediaType(img, want)
	img = mutate.ConfigMediaType(img, wantCfg)
	got, err = img.MediaType()
	if err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Errorf("%q != %q", want, got)
	}
	manifest, err = img.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.MediaType != want {
		t.Errorf("MediaType should be set for Docker media types: %v", manifest.MediaType)
	}
	if gotCfg := manifest.Config.MediaType; gotCfg != wantCfg {
		t.Errorf("manifest.Config.MediaType = %v, wanted %v", gotCfg, wantCfg)
	}

	want = types.OCIImageIndex
	idx := mutate.IndexMediaType(empty.Index, want)
	got, err = idx.MediaType()
	if err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Errorf("%q != %q", want, got)
	}
	im, err := idx.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	if im.MediaType == "" {
		t.Error("MediaType should be set for OCI media types")
	}

	want = types.DockerManifestList
	idx = mutate.IndexMediaType(idx, want)
	got, err = idx.MediaType()
	if err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Errorf("%q != %q", want, got)
	}
	im, err = idx.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	if im.MediaType != want {
		t.Errorf("MediaType should be set for Docker media types: %v", im.MediaType)
	}
}

// TestNonImageArtifactPreservesConfig verifies that mutating an OCI artifact
// with a non-Docker config type (e.g., Helm chart, WASM module) preserves the
// original config blob and correctly returns layers.
// Regression test for https://github.com/google/go-containerregistry/issues/2251
func TestNonImageArtifactPreservesConfig(t *testing.T) {
	// Start with an empty image and set a non-image config media type
	// to simulate a Helm chart or WASM artifact.
	helmConfigType := types.MediaType("application/vnd.cncf.helm.config.v1+json")
	img := mutate.ConfigMediaType(empty.Image, helmConfigType)
	img = mutate.MediaType(img, types.OCIManifestSchema1)

	// Add an annotation to trigger compute().
	img = mutate.Annotations(img, map[string]string{
		"org.opencontainers.image.title": "test-artifact",
	}).(v1.Image)

	// Layers() should not panic or return an error.
	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("Layers() failed on artifact: %v", err)
	}
	if len(layers) != 0 {
		t.Errorf("expected 0 layers for empty artifact, got %d", len(layers))
	}

	// Manifest should be valid and reference the correct config type.
	manifest, err := img.Manifest()
	if err != nil {
		t.Fatalf("Manifest() failed: %v", err)
	}
	if manifest.Config.MediaType != helmConfigType {
		t.Errorf("config media type = %v, want %v", manifest.Config.MediaType, helmConfigType)
	}

	// The config digest should not have been corrupted by re-marshaling
	// through the Docker ConfigFile struct.
	rawCfg, err := img.RawConfigFile()
	if err != nil {
		t.Fatalf("RawConfigFile() failed: %v", err)
	}
	d, _, err := v1.SHA256(bytes.NewReader(rawCfg))
	if err != nil {
		t.Fatalf("SHA256 failed: %v", err)
	}
	if manifest.Config.Digest != d {
		t.Errorf("config digest mismatch: manifest says %v, raw config hashes to %v", manifest.Config.Digest, d)
	}

	// Annotations should be present.
	if manifest.Annotations["org.opencontainers.image.title"] != "test-artifact" {
		t.Errorf("annotation not found in manifest")
	}
}

func TestAppendStreamableLayer(t *testing.T) {
	img, err := mutate.AppendLayers(
		sourceImage(t),
		stream.NewLayer(io.NopCloser(strings.NewReader(strings.Repeat("a", 100)))),
		stream.NewLayer(io.NopCloser(strings.NewReader(strings.Repeat("b", 100)))),
		stream.NewLayer(io.NopCloser(strings.NewReader(strings.Repeat("c", 100)))),
	)
	if err != nil {
		t.Fatalf("AppendLayers: %v", err)
	}

	// Until the streams are consumed, the image manifest is not yet computed.
	if _, err := img.Manifest(); !errors.Is(err, stream.ErrNotComputed) {
		t.Errorf("Manifest: got %v, want %v", err, stream.ErrNotComputed)
	}

	// We can still get Layers while some are not yet computed.
	ls, err := img.Layers()
	if err != nil {
		t.Errorf("Layers: %v", err)
	}
	wantDigests := []string{
		"sha256:bfa1c600931132f55789459e2f5a5eb85659ac91bc5a54ce09e3ed14809f8a7f",
		"sha256:77a52b9a141dcc4d3d277d053193765dca725626f50eaf56b903ac2439cf7fd1",
		"sha256:b78472d63f6e3d31059819173b56fcb0d9479a2b13c097d4addd84889f6aff06",
	}
	for i, l := range ls[1:] {
		rc, err := l.Compressed()
		if err != nil {
			t.Errorf("Layer %d Compressed: %v", i, err)
		}

		// Consume the layer's stream and close it to compute the
		// layer's metadata.
		if _, err := io.Copy(io.Discard, rc); err != nil {
			t.Errorf("Reading layer %d: %v", i, err)
		}
		if err := rc.Close(); err != nil {
			t.Errorf("Closing layer %d: %v", i, err)
		}

		// The layer's metadata is now available.
		h, err := l.Digest()
		if err != nil {
			t.Errorf("Digest after consuming layer %d: %v", i, err)
		}
		if h.String() != wantDigests[i] {
			t.Errorf("Layer %d digest got %q, want %q", i, h, wantDigests[i])
		}
	}

	// Now that the streamable layers have been consumed, the image's
	// manifest can be computed.
	if _, err := img.Manifest(); err != nil {
		t.Errorf("Manifest: %v", err)
	}

	h, err := img.Digest()
	if err != nil {
		t.Errorf("Digest: %v", err)
	}
	wantDigest := "sha256:14d140947afedc6901b490265a08bc8ebe7f9d9faed6fdf19a451f054a7dd746"
	if h.String() != wantDigest {
		t.Errorf("Image digest got %q, want %q", h, wantDigest)
	}
}

// TestExtractSymlinkFiltering verifies that Extract preserves relative symlinks
// that stay within the rootfs and absolute symlinks (left for callers to handle),
// while dropping relative symlinks whose resolved path escapes the rootfs boundary.
func TestExtractSymlinkFiltering(t *testing.T) {
	// Build a tar layer with several symlink entries.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	entries := []struct {
		name     string
		typeflag byte
		linkname string
	}{
		// Safe relative symlink: usr/local/bin/ld.so -> ../lib/ld-linux.so.2
		// resolves to usr/local/lib/ld-linux.so.2, inside rootfs.
		{name: "usr/local/bin/ld.so", typeflag: tar.TypeSymlink, linkname: "../lib/ld-linux.so.2"},
		// Safe relative symlink: var/lock -> ../run/lock (tailscale/glibc pattern).
		{name: "usr/local/lib/containers/app/var/lock", typeflag: tar.TypeSymlink, linkname: "../run/lock"},
		// Absolute target: preserved (see #2238 for ongoing discussion).
		{name: "usr/local/bin/abs-link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"},
		// Unsafe: relative target resolves outside rootfs.
		{name: "etc/foo", typeflag: tar.TypeSymlink, linkname: "../../../tmp/evil"},
	}
	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Typeflag: e.typeflag,
			Name:     e.name,
			Linkname: e.linkname,
		}); err != nil {
			t.Fatalf("WriteHeader: %v", err)
		}
	}
	tw.Close()

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})
	if err != nil {
		t.Fatalf("LayerFromOpener: %v", err)
	}
	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		t.Fatalf("AppendLayers: %v", err)
	}

	extracted := map[string]string{}
	tr := tar.NewReader(mutate.Extract(img))
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("reading extracted tar: %v", err)
		}
		if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			extracted[hdr.Name] = hdr.Linkname
		}
	}

	// These symlinks must be preserved: safe relative ones and absolute ones.
	for _, name := range []string{"usr/local/bin/ld.so", "usr/local/lib/containers/app/var/lock", "usr/local/bin/abs-link"} {
		if _, ok := extracted[name]; !ok {
			t.Errorf("symlink %q was incorrectly dropped", name)
		}
	}
	// Relative targets that escape the rootfs must be filtered out.
	for _, name := range []string{"etc/foo"} {
		if target, ok := extracted[name]; ok {
			t.Errorf("unsafe symlink %q -> %q was not dropped", name, target)
		}
	}
}

func TestCanonical(t *testing.T) {
	source := sourceImage(t)
	img, err := mutate.Canonical(source)
	if err != nil {
		t.Fatal(err)
	}
	sourceCf, err := source.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	cf, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range cf.History {
		want := "bazel build ..."
		got := h.CreatedBy
		if want != got {
			t.Errorf("%q != %q", want, got)
		}
	}
	var want, got string
	want = cf.Architecture
	got = sourceCf.Architecture
	if want != got {
		t.Errorf("%q != %q", want, got)
	}
	want = cf.OS
	got = sourceCf.OS
	if want != got {
		t.Errorf("%q != %q", want, got)
	}
	want = cf.OSVersion
	got = sourceCf.OSVersion
	if want != got {
		t.Errorf("%q != %q", want, got)
	}
	for _, s := range []string{
		cf.Container,
		cf.Config.Hostname,
		cf.DockerVersion, //nolint:staticcheck // Field will be removed in next release
	} {
		if s != "" {
			t.Errorf("non-zeroed string: %v", s)
		}
	}

	expectedLayerTime := time.Unix(0, 0)
	layers := getLayers(t, img)
	for _, layer := range layers {
		assertMTime(t, layer, expectedLayerTime)
	}
}

func TestRemoveManifests(t *testing.T) {
	// Load up the registry.
	count := 3
	for i := 0; i < count; i++ {
		ii, err := random.Index(1024, int64(count), int64(count))
		if err != nil {
			t.Fatal(err)
		}
		// test removing the first layer, second layer or the third layer
		manifest, err := ii.IndexManifest()
		if err != nil {
			t.Fatal(err)
		}
		if len(manifest.Manifests) != count {
			t.Fatalf("mismatched manifests on setup, had %d, expected %d", len(manifest.Manifests), count)
		}
		digest := manifest.Manifests[i].Digest
		ii = mutate.RemoveManifests(ii, match.Digests(digest))
		manifest, err = ii.IndexManifest()
		if err != nil {
			t.Fatal(err)
		}
		if len(manifest.Manifests) != (count - 1) {
			t.Fatalf("mismatched manifests after removal, had %d, expected %d", len(manifest.Manifests), count-1)
		}
		for j, m := range manifest.Manifests {
			if m.Digest == digest {
				t.Fatalf("unexpectedly found removed hash %v at position %d", digest, j)
			}
		}
	}
}

func TestImageImmutability(t *testing.T) {
	img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)

	t.Run("manifest", func(t *testing.T) {
		// Check that Manifest is immutable.
		changed, err := img.Manifest()
		if err != nil {
			t.Errorf("Manifest() = %v", err)
		}
		want := changed.DeepCopy() // Create a copy of original before mutating it.
		changed.MediaType = types.DockerManifestList

		if got, err := img.Manifest(); err != nil {
			t.Errorf("Manifest() = %v", err)
		} else if !cmp.Equal(got, want) {
			t.Errorf("manifest changed! %s", cmp.Diff(got, want))
		}
	})

	t.Run("config file", func(t *testing.T) {
		// Check that ConfigFile is immutable.
		changed, err := img.ConfigFile()
		if err != nil {
			t.Errorf("ConfigFile() = %v", err)
		}
		want := changed.DeepCopy() // Create a copy of original before mutating it.
		changed.Author = "Jay Pegg"

		if got, err := img.ConfigFile(); err != nil {
			t.Errorf("ConfigFile() = %v", err)
		} else if !cmp.Equal(got, want) {
			t.Errorf("ConfigFile changed! %s", cmp.Diff(got, want))
		}
	})
}

func assertMTime(t *testing.T, layer v1.Layer, expectedTime time.Time) {
	l, err := layer.Uncompressed()

	if err != nil {
		t.Fatalf("reading layer failed: %v", err)
	}

	tr := tar.NewReader(l)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Error reading layer: %v", err)
		}

		mtime := header.ModTime
		if mtime.Equal(expectedTime) == false {
			t.Errorf("unexpected mod time for layer. expected %v, got %v.", expectedTime, mtime)
		}
	}
}

func sourceImage(t *testing.T) v1.Image {
	return sourceImagePath(t, "testdata/source_image.tar")
}

func sourceImagePath(t *testing.T, tarPath string) v1.Image {
	t.Helper()

	image, err := tarball.ImageFromPath(tarPath, nil)
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

func configDigestsAreEqual(t *testing.T, first, second v1.Image) bool {
	t.Helper()

	fm := getManifest(t, first)
	sm := getManifest(t, second)

	return fm.Config.Digest == sm.Config.Digest
}

func configSizesAreEqual(t *testing.T, first, second v1.Image) bool {
	t.Helper()

	fm := getManifest(t, first)
	sm := getManifest(t, second)

	return fm.Config.Size == sm.Config.Size
}

func manifestsAreEqual(t *testing.T, first, second v1.Image) bool {
	t.Helper()

	fm := getManifest(t, first)
	sm := getManifest(t, second)

	return cmp.Equal(fm, sm)
}

type mockLayer struct{}

func (m mockLayer) Digest() (v1.Hash, error) {
	return v1.Hash{Algorithm: "fake", Hex: "digest"}, nil
}

func (m mockLayer) DiffID() (v1.Hash, error) {
	return v1.Hash{Algorithm: "fake", Hex: "diff id"}, nil
}

func (m mockLayer) MediaType() (types.MediaType, error) {
	return "some-media-type", nil
}

func (m mockLayer) Size() (int64, error) { return 137438691328, nil }
func (m mockLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("compressed times")), nil
}
func (m mockLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("uncompressed")), nil
}

type tokenLimitedImage struct {
	v1.Image
	layers []v1.Layer
}

func (i tokenLimitedImage) Layers() ([]v1.Layer, error) {
	return i.layers, nil
}

type tokenLimitedLayer struct {
	mockLayer
	tokens chan struct{}
}

func (l tokenLimitedLayer) Uncompressed() (io.ReadCloser, error) {
	select {
	case l.tokens <- struct{}{}:
		return &tokenReleasingReadCloser{
			Reader:  bytes.NewReader(nil),
			release: func() { <-l.tokens },
		}, nil
	default:
		return nil, errors.New("layer reader opened before previous readers were closed")
	}
}

type tokenReleasingReadCloser struct {
	io.Reader
	release func()
	closed  bool
}

func (rc *tokenReleasingReadCloser) Close() error {
	if !rc.closed {
		rc.closed = true
		rc.release()
	}
	return nil
}

// duplicateDiffIDLayer wraps a v1.Layer so that the wrapper reports the
// inner layer's DiffID but its own (distinct) Digest, Size, and Compressed
// stream. This models the real-world case behind #2034: two layers built
// from the same uncompressed tar but with different compression settings
// share a diff ID and have distinct blob digests.
type duplicateDiffIDLayer struct {
	v1.Layer
	digest     v1.Hash
	compressed []byte
}

func (d *duplicateDiffIDLayer) Digest() (v1.Hash, error) { return d.digest, nil }
func (d *duplicateDiffIDLayer) Size() (int64, error)     { return int64(len(d.compressed)), nil }
func (d *duplicateDiffIDLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(d.compressed)), nil
}

// TestAppendLayers_DuplicateDiffID is a regression test for #2034: when a
// base image layer and an appended layer share a diff ID but have different
// blob digests, Layers() must return both as distinct entries (one per
// manifest descriptor). Walking the rootfs diff IDs and resolving each via
// LayerByDiffID — the old behavior — collapsed the duplicate-diff-ID entry
// to a single layer, which broke downstream pushers that uploaded blobs
// based on Layers() (resulting in MANIFEST_BLOB_UNKNOWN at push time).
func TestAppendLayers_DuplicateDiffID(t *testing.T) {
	base, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	baseLayers, err := base.Layers()
	if err != nil {
		t.Fatal(err)
	}
	baseLayer := baseLayers[0]

	baseDigest, err := baseLayer.Digest()
	if err != nil {
		t.Fatal(err)
	}
	baseDiffID, err := baseLayer.DiffID()
	if err != nil {
		t.Fatal(err)
	}

	// Construct an appended layer that reports baseLayer's diff ID but a
	// fabricated, distinct digest. We don't care that the bytes are
	// "really" a valid recompression — the bug is in how Layers() resolves
	// per-occurrence layers from the manifest, not how the blob is
	// validated.
	appendBlob := []byte("a different compressed blob for the same uncompressed content")
	fakeDigest := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("ab", 32)}
	if fakeDigest == baseDigest {
		t.Fatalf("test setup invariant broken: fabricated digest collides with base")
	}
	appended := &duplicateDiffIDLayer{
		Layer:      baseLayer,
		digest:     fakeDigest,
		compressed: appendBlob,
	}

	result, err := mutate.AppendLayers(base, appended)
	if err != nil {
		t.Fatalf("AppendLayers failed: %v", err)
	}

	// Quick check: the manifest must list both digests in order.
	m, err := result.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(m.Layers), 2; got != want {
		t.Fatalf("manifest layers: got %d, want %d", got, want)
	}
	if m.Layers[0].Digest != baseDigest {
		t.Errorf("manifest layers[0].Digest: got %s, want %s", m.Layers[0].Digest, baseDigest)
	}
	if m.Layers[1].Digest != fakeDigest {
		t.Errorf("manifest layers[1].Digest: got %s, want %s", m.Layers[1].Digest, fakeDigest)
	}

	// The bug: Layers() previously walked diffIDs and resolved via
	// LayerByDiffID, which collapsed both entries to the appended layer.
	// Both slots returned a layer with digest=fakeDigest.
	layers, err := result.Layers()
	if err != nil {
		t.Fatalf("Layers() failed: %v", err)
	}
	if got, want := len(layers), 2; got != want {
		t.Fatalf("Layers(): got %d, want %d", got, want)
	}

	d0, err := layers[0].Digest()
	if err != nil {
		t.Fatal(err)
	}
	if d0 != baseDigest {
		t.Errorf("Layers()[0].Digest(): got %s, want %s (base) — duplicate-diff-ID collapse regression", d0, baseDigest)
	}

	d1, err := layers[1].Digest()
	if err != nil {
		t.Fatal(err)
	}
	if d1 != fakeDigest {
		t.Errorf("Layers()[1].Digest(): got %s, want %s (appended)", d1, fakeDigest)
	}

	// Round-trip check: both layers must also be retrievable via
	// LayerByDigest using the digests reported in the manifest.
	if _, err := result.LayerByDigest(baseDigest); err != nil {
		t.Errorf("LayerByDigest(base): %v", err)
	}
	if _, err := result.LayerByDigest(fakeDigest); err != nil {
		t.Errorf("LayerByDigest(appended): %v", err)
	}

	// The diff IDs must match — confirming the test setup actually
	// exercises the duplicate-diff-ID case.
	id0, err := layers[0].DiffID()
	if err != nil {
		t.Fatal(err)
	}
	id1, err := layers[1].DiffID()
	if err != nil {
		t.Fatal(err)
	}
	if id0 != id1 {
		t.Fatalf("test setup invariant: expected duplicate diff IDs, got %s and %s", id0, id1)
	}
	if id0 != baseDiffID {
		t.Fatalf("test setup invariant: expected base diff ID %s, got %s", baseDiffID, id0)
	}
}

// makeTarBytes builds a single-file tar archive in memory and returns the
// raw (uncompressed) tar bytes.
func makeTarBytes(t *testing.T, name, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	return buf.Bytes()
}

// verifyingLayer is a v1.Layer whose Uncompressed stream is wrapped by
// internal/verify against expectDiffID -- exactly how layers loaded from a
// tarball or a registry are constructed. When expectDiffID does not match
// the bytes in tarBytes, the layer models content that disagrees with the
// digest recorded in the image manifest.
type verifyingLayer struct {
	v1.Layer
	tarBytes     []byte
	expectDiffID v1.Hash
}

func (l *verifyingLayer) Uncompressed() (io.ReadCloser, error) {
	return verify.ReadCloser(io.NopCloser(bytes.NewReader(l.tarBytes)), int64(len(l.tarBytes)), l.expectDiffID)
}

// layersImage overrides Layers() on an embedded image so mutate operations
// see the supplied layers instead of the real ones.
type layersImage struct {
	v1.Image
	layers []v1.Layer
}

func (i layersImage) Layers() ([]v1.Layer, error) { return i.layers, nil }

// TestExtractVerifiesLayerDigest is a regression test for layer-digest
// verification in mutate.Extract. Extract reads each layer through a
// tar.Reader, which stops at the tar end-of-archive marker before the
// underlying verifying reader reaches io.EOF. The digest check in
// internal/verify only fires at io.EOF, so without an explicit drain Extract
// accepts a layer whose contents do not match the manifest's layer digest.
// crane export / crane edit build on Extract and inherit the same path.
func TestExtractVerifiesLayerDigest(t *testing.T) {
	tarBytes := makeTarBytes(t, "app/hello.txt", "hello world")
	goodDiffID, _, err := v1.SHA256(bytes.NewReader(tarBytes))
	if err != nil {
		t.Fatal(err)
	}
	// A well-formed but incorrect digest: same algorithm, different hex.
	badDiffID := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("00", 32)}
	if badDiffID == goodDiffID {
		t.Fatal("test setup: fabricated digest collides with real digest")
	}

	t.Run("digest mismatch is rejected", func(t *testing.T) {
		img := layersImage{layers: []v1.Layer{
			&verifyingLayer{tarBytes: tarBytes, expectDiffID: badDiffID},
		}}
		_, err := io.Copy(io.Discard, mutate.Extract(img))
		if err == nil {
			t.Fatal("Extract accepted a layer whose contents do not match its digest")
		}
		if !strings.Contains(err.Error(), "checksum") {
			t.Fatalf("Extract error = %v, want a digest verification error", err)
		}
	})

	t.Run("matching digest extracts cleanly", func(t *testing.T) {
		img := layersImage{layers: []v1.Layer{
			&verifyingLayer{tarBytes: tarBytes, expectDiffID: goodDiffID},
		}}
		tr := tar.NewReader(mutate.Extract(img))
		var names []string
		for {
			hdr, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("Extract of honest layer failed: %v", err)
			}
			names = append(names, hdr.Name)
		}
		if len(names) != 1 || names[0] != "app/hello.txt" {
			t.Fatalf("extracted entries = %v, want [app/hello.txt]", names)
		}
	})
}

// TestTimeVerifiesLayerDigest is the same regression test for mutate.Time,
// which reads layers through layerTime using the same early-stopping
// tar.Reader loop.
func TestTimeVerifiesLayerDigest(t *testing.T) {
	tarBytes := makeTarBytes(t, "app/hello.txt", "hello world")
	goodDiffID, _, err := v1.SHA256(bytes.NewReader(tarBytes))
	if err != nil {
		t.Fatal(err)
	}
	badDiffID := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("00", 32)}
	if badDiffID == goodDiffID {
		t.Fatal("test setup: fabricated digest collides with real digest")
	}

	// Time needs a config file; borrow one from a random single-layer image
	// and override only Layers().
	base, err := random.Image(256, 1)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("digest mismatch is rejected", func(t *testing.T) {
		img := layersImage{Image: base, layers: []v1.Layer{
			&verifyingLayer{tarBytes: tarBytes, expectDiffID: badDiffID},
		}}
		if _, err := mutate.Time(img, time.Unix(0, 0)); err == nil {
			t.Fatal("Time accepted a layer whose contents do not match its digest")
		} else if !strings.Contains(err.Error(), "checksum") {
			t.Fatalf("Time error = %v, want a digest verification error", err)
		}
	})

	t.Run("matching digest succeeds", func(t *testing.T) {
		img := layersImage{Image: base, layers: []v1.Layer{
			&verifyingLayer{tarBytes: tarBytes, expectDiffID: goodDiffID},
		}}
		if _, err := mutate.Time(img, time.Unix(0, 0)); err != nil {
			t.Fatalf("Time of honest layer failed: %v", err)
		}
	})
}
