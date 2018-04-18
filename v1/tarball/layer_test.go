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
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/v1"
)

func TestLayerFromFile(t *testing.T) {
	setupFixtures(t)

	tarLayer, err := LayerFromFile("content.tar")
	if err != nil {
		t.Fatalf("Unable to create layer from tar file: %v", err)
	}

	tarGzLayer, err := LayerFromFile("gzip_content.tgz")
	if err != nil {
		t.Fatalf("Unable to create layer from compressed tar file: %v", err)
	}

	assertDigestsAreEqual(t, tarLayer, tarGzLayer)
	assertDiffIDsAreEqual(t, tarLayer, tarGzLayer)
	assertCompressedStreamsAreEqual(t, tarLayer, tarGzLayer)
	assertUncompressedStreamsAreEqual(t, tarLayer, tarGzLayer)
	assertSizesAreEqual(t, tarLayer, tarGzLayer)
}

func assertDigestsAreEqual(t *testing.T, a, b v1.Layer) {
	t.Helper()

	sa, err := a.Digest()
	if err != nil {
		t.Fatalf("Unable to fetch digest for layer: %v", err)
	}

	sb, err := b.Digest()
	if err != nil {
		t.Fatalf("Unable to fetch digest for layer: %v", err)
	}

	if sa != sb {
		t.Fatalf("Digest of each layer is different - %v != %v", sa, sb)
	}
}

func assertDiffIDsAreEqual(t *testing.T, a, b v1.Layer) {
	t.Helper()

	sa, err := a.DiffID()
	if err != nil {
		t.Fatalf("Unable to fetch diffID for layer: %v", err)
	}

	sb, err := b.DiffID()
	if err != nil {
		t.Fatalf("Unable to fetch diffID for layer: %v", err)
	}

	if sa != sb {
		t.Fatalf("diffID of each layer is different - %v != %v", sa, sb)
	}
}

func assertCompressedStreamsAreEqual(t *testing.T, a, b v1.Layer) {
	t.Helper()

	sa, err := a.Compressed()
	if err != nil {
		t.Fatalf("Unable to fetch compressed for layer: %v", err)
	}

	saBytes, err := ioutil.ReadAll(sa)
	if err != nil {
		t.Fatalf("Unable to read bytes for layer: %v", err)
	}

	sb, err := b.Compressed()
	if err != nil {
		t.Fatalf("Unable to fetch compressed for layer: %v", err)
	}

	sbBytes, err := ioutil.ReadAll(sb)
	if err != nil {
		t.Fatalf("Unable to read bytes for layer: %v", err)
	}

	if diff := cmp.Diff(saBytes, sbBytes); diff != "" {
		t.Fatalf("Compressed streams were different: %v", diff)
	}
}

func assertUncompressedStreamsAreEqual(t *testing.T, a, b v1.Layer) {
	t.Helper()

	sa, err := a.Uncompressed()
	if err != nil {
		t.Fatalf("Unable to fetch uncompressed for layer: %v", err)
	}

	saBytes, err := ioutil.ReadAll(sa)
	if err != nil {
		t.Fatalf("Unable to read bytes for layer: %v", err)
	}

	sb, err := b.Uncompressed()
	if err != nil {
		t.Fatalf("Unable to fetch uncompressed for layer: %v", err)
	}

	sbBytes, err := ioutil.ReadAll(sb)
	if err != nil {
		t.Fatalf("Unable to read bytes for layer: %v", err)
	}

	if diff := cmp.Diff(saBytes, sbBytes); diff != "" {
		t.Fatalf("Uncompressed streams were different: %v", diff)
	}
}

func assertSizesAreEqual(t *testing.T, a, b v1.Layer) {
	t.Helper()

	sa, err := a.Size()
	if err != nil {
		t.Fatalf("Unable to fetch size for layer: %v", err)
	}

	sb, err := b.Size()
	if err != nil {
		t.Fatalf("Unable to fetch size for layer: %v", err)
	}

	if sa != sb {
		t.Fatalf("Size of each layer is different - %d != %d", sa, sb)
	}
}

// Compression settings matter in order for the digest, size,
// compressed assertions to pass
//
// Since our v1util.GzipReadCloser uses gzip.BestCompression
// we need our fixture to use the same - bazel's pkg_tar doesn't
// seem to let you control compression settings
func setupFixtures(t *testing.T) {
	t.Helper()

	in, err := os.Open("content.tar")
	if err != nil {
		t.Errorf("Error setting up fixtures: %v", err)
	}

	defer in.Close()

	out, err := os.Create("gzip_content.tgz")
	if err != nil {
		t.Errorf("Error setting up fixtures: %v", err)
	}

	defer out.Close()

	gw, _ := gzip.NewWriterLevel(out, gzip.BestCompression)
	defer gw.Close()

	_, err = io.Copy(gw, in)
	if err != nil {
		t.Errorf("Error setting up fixtures: %v", err)
	}
}
