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

package tarball

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"testing"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/google/go-containerregistry/internal/compare"
	"github.com/google/go-containerregistry/pkg/compression"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestLayerFromFile(t *testing.T) {
	setupFixtures(t)
	defer teardownFixtures(t)

	tarLayer, err := LayerFromFile("testdata/content.tar")
	if err != nil {
		t.Fatalf("Unable to create layer from tar file: %v", err)
	}

	tarGzLayer, err := LayerFromFile("gzip_content.tgz")
	if err != nil {
		t.Fatalf("Unable to create layer from compressed tar file: %v", err)
	}

	tarZstdLayer, err := LayerFromFile("zstd_content.tar.zst")
	if err != nil {
		t.Fatalf("Unable to create layer from compressed tar file: %v", err)
	}

	if err := compare.Layers(tarLayer, tarGzLayer); err != nil {
		t.Errorf("compare.Layers: %v", err)
	}

	if err := compare.Layers(tarLayer, tarZstdLayer); err != nil {
		t.Errorf("compare.Layers: %v", err)
	}

	if err := validate.Layer(tarLayer); err != nil {
		t.Errorf("validate.Layer(tarLayer): %v", err)
	}

	if err := validate.Layer(tarGzLayer); err != nil {
		t.Errorf("validate.Layer(tarGzLayer): %v", err)
	}

	if err := validate.Layer(tarZstdLayer); err != nil {
		t.Errorf("validate.Layer(tarZstdLayer): %v", err)
	}

	getTestDigest := func(testName string, opts ...LayerOption) v1.Hash {
		layer, err := LayerFromFile("testdata/content.tar", opts...)
		if err != nil {
			t.Fatalf("Unable to create layer with '%s' compression from tar file: %v", testName, err)
		}

		digest, err := layer.Digest()
		if err != nil {
			t.Fatalf("Unable to generate digest with '%s' compression: %v", testName, err)
		}

		return digest
	}

	defaultDigest := getTestDigest("Gzip Default", WithCompressionLevel(gzip.DefaultCompression))
	speedDigest := getTestDigest("Gzip BestSpeed", WithCompressionLevel(gzip.BestSpeed))
	zstdDigest := getTestDigest("Zstd Default", WithCompression(compression.ZStd))
	zstdDigest1 := getTestDigest("Zstd BestSpeed", WithCompression(compression.ZStd), WithCompressionLevel(1))

	if defaultDigest.String() == speedDigest.String() {
		t.Errorf("expected digests to differ: %s", defaultDigest.String())
	}

	if defaultDigest.String() == zstdDigest.String() {
		t.Errorf("expected digests to differ: %s", defaultDigest.String())
	}

	if defaultDigest.String() == zstdDigest1.String() {
		t.Errorf("expected digests to differ: %s", defaultDigest.String())
	}
}

func TestLayerFromFileEstargz(t *testing.T) {
	setupFixtures(t)
	defer teardownFixtures(t)

	tarLayer, err := LayerFromFile("testdata/content.tar", WithEstargz)
	if err != nil {
		t.Fatalf("Unable to create layer from tar file: %v", err)
	}

	if err := validate.Layer(tarLayer); err != nil {
		t.Errorf("validate.Layer(tarLayer): %v", err)
	}

	tarLayerDefaultCompression, err := LayerFromFile("testdata/content.tar", WithEstargz, WithCompressionLevel(gzip.DefaultCompression))
	if err != nil {
		t.Fatalf("Unable to create layer with 'Default' compression from tar file: %v", err)
	}
	descriptorDefaultCompression, err := tarLayerDefaultCompression.(*layer).Descriptor()
	if err != nil {
		t.Fatalf("Descriptor() = %v", err)
	} else if len(descriptorDefaultCompression.Annotations) != 1 {
		t.Errorf("Annotations = %#v, wanted 1 annotation", descriptorDefaultCompression.Annotations)
	}

	defaultDigest, err := tarLayerDefaultCompression.Digest()
	if err != nil {
		t.Fatal("Unable to generate digest with 'Default' compression", err)
	}

	tarLayerSpeedCompression, err := LayerFromFile("testdata/content.tar", WithEstargz, WithCompressionLevel(gzip.BestSpeed))
	if err != nil {
		t.Fatalf("Unable to create layer with 'BestSpeed' compression from tar file: %v", err)
	}
	descriptorSpeedCompression, err := tarLayerSpeedCompression.(*layer).Descriptor()
	if err != nil {
		t.Fatalf("Descriptor() = %v", err)
	} else if len(descriptorSpeedCompression.Annotations) != 1 {
		t.Errorf("Annotations = %#v, wanted 1 annotation", descriptorSpeedCompression.Annotations)
	}

	speedDigest, err := tarLayerSpeedCompression.Digest()
	if err != nil {
		t.Fatal("Unable to generate digest with 'BestSpeed' compression", err)
	}

	if defaultDigest.String() == speedDigest.String() {
		t.Errorf("expected digests to differ: %s", defaultDigest.String())
	}

	if descriptorDefaultCompression.Annotations[estargz.TOCJSONDigestAnnotation] == descriptorSpeedCompression.Annotations[estargz.TOCJSONDigestAnnotation] {
		t.Errorf("wanted different toc digests got default: %s, speed: %s",
			descriptorDefaultCompression.Annotations[estargz.TOCJSONDigestAnnotation],
			descriptorSpeedCompression.Annotations[estargz.TOCJSONDigestAnnotation])
	}

	tarLayerPrioritizedFiles, err := LayerFromFile("testdata/content.tar",
		WithEstargz,
		// We compare with default, so pass for apples-to-apples comparison.
		WithCompressionLevel(gzip.DefaultCompression),
		// By passing a list of priority files, we expect the layer to be different.
		WithEstargzOptions(estargz.WithPrioritizedFiles([]string{
			"./bat",
		})))
	if err != nil {
		t.Fatalf("Unable to create layer with prioritized files from tar file: %v", err)
	}
	descriptorPrioritizedFiles, err := tarLayerPrioritizedFiles.(*layer).Descriptor()
	if err != nil {
		t.Fatalf("Descriptor() = %v", err)
	} else if len(descriptorPrioritizedFiles.Annotations) != 1 {
		t.Errorf("Annotations = %#v, wanted 1 annotation", descriptorPrioritizedFiles.Annotations)
	}

	prioritizedDigest, err := tarLayerPrioritizedFiles.Digest()
	if err != nil {
		t.Fatal("Unable to generate digest with prioritized files", err)
	}

	if defaultDigest.String() == prioritizedDigest.String() {
		t.Errorf("expected digests to differ: %s", defaultDigest.String())
	}

	if descriptorDefaultCompression.Annotations[estargz.TOCJSONDigestAnnotation] == descriptorPrioritizedFiles.Annotations[estargz.TOCJSONDigestAnnotation] {
		t.Errorf("wanted different toc digests got default: %s, prioritized: %s",
			descriptorDefaultCompression.Annotations[estargz.TOCJSONDigestAnnotation],
			descriptorPrioritizedFiles.Annotations[estargz.TOCJSONDigestAnnotation])
	}
}

func TestLayerFromOpenerReader(t *testing.T) {
	setupFixtures(t)
	defer teardownFixtures(t)

	ucBytes, err := os.ReadFile("testdata/content.tar")
	if err != nil {
		t.Fatalf("Unable to read tar file: %v", err)
	}
	count := 0
	ucOpener := func() (io.ReadCloser, error) {
		count++
		return io.NopCloser(bytes.NewReader(ucBytes)), nil
	}
	tarLayer, err := LayerFromOpener(ucOpener, WithCompressedCaching)
	if err != nil {
		t.Fatal("Unable to create layer from tar file:", err)
	}
	for i := 0; i < 10; i++ {
		tarLayer.Compressed()
	}

	// Store the count and reset the counter.
	cachedCount := count
	count = 0

	tarLayer, err = LayerFromOpener(ucOpener)
	if err != nil {
		t.Fatal("Unable to create layer from tar file:", err)
	}
	for i := 0; i < 10; i++ {
		tarLayer.Compressed()
	}

	// We expect three calls: gzip sniff, diffid computation, cached compression
	if cachedCount != 3 {
		t.Errorf("cached count = %d, wanted %d", cachedCount, 3)
	}
	if cachedCount+10 != count {
		t.Errorf("count = %d, wanted %d", count, cachedCount+10)
	}

	gzBytes, err := os.ReadFile("gzip_content.tgz")
	if err != nil {
		t.Fatalf("Unable to read tar file: %v", err)
	}
	gzOpener := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(gzBytes)), nil
	}
	tarGzLayer, err := LayerFromOpener(gzOpener)
	if err != nil {
		t.Fatalf("Unable to create layer from tar file: %v", err)
	}

	if err := compare.Layers(tarLayer, tarGzLayer); err != nil {
		t.Errorf("compare.Layers: %v", err)
	}

	zstdBytes, err := os.ReadFile("zstd_content.tar.zst")
	if err != nil {
		t.Fatalf("Unable to read tar file: %v", err)
	}
	zstdOpener := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(zstdBytes)), nil
	}
	tarZstdLayer, err := LayerFromOpener(zstdOpener)
	if err != nil {
		t.Fatalf("Unable to create layer from tar file: %v", err)
	}

	if err := compare.Layers(tarLayer, tarZstdLayer); err != nil {
		t.Errorf("compare.Layers: %v", err)
	}
}

func TestWithMediaType(t *testing.T) {
	setupFixtures(t)
	defer teardownFixtures(t)

	l, err := LayerFromFile("testdata/content.tar")
	if err != nil {
		t.Fatalf("Unable to create layer from tar file: %v", err)
	}
	got, err := l.MediaType()
	if err != nil {
		t.Fatalf("MediaType: %v", err)
	}
	if want := types.DockerLayer; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	l, err = LayerFromFile("testdata/content.tar", WithMediaType(types.OCILayer))
	if err != nil {
		t.Fatalf("Unable to create layer from tar file: %v", err)
	}
	got, err = l.MediaType()
	if err != nil {
		t.Fatalf("MediaType: %v", err)
	}
	if want := types.OCILayer; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLayerFromReader(t *testing.T) {
	setupFixtures(t)
	defer teardownFixtures(t)

	ucBytes, err := os.ReadFile("testdata/content.tar")
	if err != nil {
		t.Fatalf("Unable to read tar file: %v", err)
	}
	tarLayer, err := LayerFromReader(bytes.NewReader(ucBytes))
	if err != nil {
		t.Fatalf("Unable to create layer from tar file: %v", err)
	}

	gzBytes, err := os.ReadFile("gzip_content.tgz")
	if err != nil {
		t.Fatalf("Unable to read tar file: %v", err)
	}
	tarGzLayer, err := LayerFromReader(bytes.NewReader(gzBytes))
	if err != nil {
		t.Fatalf("Unable to create layer from tar file: %v", err)
	}

	if err := compare.Layers(tarLayer, tarGzLayer); err != nil {
		t.Errorf("compare.Layers: %v", err)
	}

	zstdBytes, err := os.ReadFile("zstd_content.tar.zst")
	if err != nil {
		t.Fatalf("Unable to read tar file: %v", err)
	}
	tarZstdLayer, err := LayerFromReader(bytes.NewReader(zstdBytes))
	if err != nil {
		t.Fatalf("Unable to create layer from tar file: %v", err)
	}

	if err := compare.Layers(tarLayer, tarZstdLayer); err != nil {
		t.Errorf("compare.Layers: %v", err)
	}
}

// Compression settings matter in order for the digest, size,
// compressed assertions to pass
//
// Since our gzip.GzipReadCloser uses gzip.BestSpeed
// we need our fixture to use the same - bazel's pkg_tar doesn't
// seem to let you control compression settings
func setupFixtures(t *testing.T) {
	t.Helper()

	setupCompressedTar(t, "gzip_content.tgz")
	setupCompressedTar(t, "zstd_content.tar.zst")
}

func setupCompressedTar(t *testing.T, fileName string) {
	t.Helper()
	in, err := os.Open("testdata/content.tar")
	if err != nil {
		t.Errorf("Error setting up fixtures: %v", err)
	}

	defer in.Close()

	out, err := os.Create(fileName)
	if err != nil {
		t.Errorf("Error setting up fixtures: %v", err)
	}

	defer out.Close()

	gw, _ := gzip.NewWriterLevel(out, gzip.BestSpeed)
	defer gw.Close()

	_, err = io.Copy(gw, in)
	if err != nil {
		t.Errorf("Error setting up fixtures: %v", err)
	}
}

func teardownFixtures(t *testing.T) {
	t.Helper()
	if err := os.Remove("gzip_content.tgz"); err != nil {
		t.Errorf("Error tearing down fixtures: %v", err)
	}
	if err := os.Remove("zstd_content.tar.zst"); err != nil {
		t.Errorf("Error tearing down fixtures: %v", err)
	}
}
