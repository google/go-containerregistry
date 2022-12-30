// Copyright 2022 Google LLC All Rights Reserved.
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

package layout

import (
	"archive/tar"
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestWrite(t *testing.T) {
	tmp := t.TempDir()

	original, err := ImageIndexFromPath(testPath)
	if err != nil {
		t.Fatal(err)
	}

	if layoutPath, err := Write(tmp, original); err != nil {
		t.Fatalf("Write(%s) = %v", tmp, err)
	} else if tmp != layoutPath.path() {
		t.Fatalf("unexpected file system path %v", layoutPath)
	}

	written, err := ImageIndexFromPath(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if err := validate.Index(written); err != nil {
		t.Fatalf("validate.Index() = %v", err)
	}
}

func TestWriteErrors(t *testing.T) {
	idx, err := ImageIndexFromPath(testPath)
	if err != nil {
		t.Fatalf("ImageIndexFromPath() = %v", err)
	}

	// Found this here:
	// https://github.com/golang/go/issues/24195
	invalidPath := "double-null-padded-string\x00\x00"
	if _, err := Write(invalidPath, idx); err == nil {
		t.Fatalf("Write(%s) = nil, expected err", invalidPath)
	}
}

func TestAppendDescriptorInitializesIndex(t *testing.T) {
	tmp := t.TempDir()
	temp, err := Write(tmp, empty.Index)
	if err != nil {
		t.Fatal(err)
	}

	// Append a descriptor to a non-existent layout.
	desc := v1.Descriptor{
		Digest:    bogusDigest,
		Size:      1337,
		MediaType: types.MediaType("not real"),
	}
	if err := temp.AppendDescriptor(desc); err != nil {
		t.Fatalf("AppendDescriptor(%s) = %v", tmp, err)
	}

	// Read that layout from disk and make sure the descriptor is there.
	idx, err := ImageIndexFromPath(tmp)
	if err != nil {
		t.Fatalf("ImageIndexFromPath() = %v", err)
	}

	manifest, err := idx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest() = %v", err)
	}
	if diff := cmp.Diff(manifest.Manifests[0], desc); diff != "" {
		t.Fatalf("bad descriptor: (-got +want) %s", diff)
	}
}

func TestRoundtrip(t *testing.T) {
	tmp := t.TempDir()

	original, err := ImageIndexFromPath(testPath)
	if err != nil {
		t.Fatal(err)
	}

	originalManifest, err := original.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}

	// Write it back.
	if _, err := Write(tmp, original); err != nil {
		t.Fatal(err)
	}
	reconstructed, err := ImageIndexFromPath(tmp)
	if err != nil {
		t.Fatalf("ImageIndexFromPath() = %v", err)
	}
	reconstructedManifest, err := reconstructed.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(originalManifest, reconstructedManifest); diff != "" {
		t.Fatalf("bad manifest: (-got +want) %s", diff)
	}
}

func TestOptions(t *testing.T) {
	tmp := t.TempDir()
	temp, err := Write(tmp, empty.Index)
	if err != nil {
		t.Fatal(err)
	}
	annotations := map[string]string{
		"foo": "bar",
	}
	urls := []string{"https://example.com"}
	platform := v1.Platform{
		Architecture: "mill",
		OS:           "haiku",
	}
	img, err := random.Image(5, 5)
	if err != nil {
		t.Fatal(err)
	}
	options := []Option{
		WithAnnotations(annotations),
		WithURLs(urls),
		WithPlatform(platform),
	}
	if err := temp.AppendImage(img, options...); err != nil {
		t.Fatal(err)
	}
	idx, err := temp.ImageIndex()
	if err != nil {
		t.Fatal(err)
	}
	indexManifest, err := idx.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}

	desc := indexManifest.Manifests[0]
	if got, want := desc.Annotations["foo"], "bar"; got != want {
		t.Fatalf("wrong annotation; got: %v, want: %v", got, want)
	}
	if got, want := desc.URLs[0], "https://example.com"; got != want {
		t.Fatalf("wrong urls; got: %v, want: %v", got, want)
	}
	if got, want := desc.Platform.Architecture, "mill"; got != want {
		t.Fatalf("wrong Architecture; got: %v, want: %v", got, want)
	}
	if got, want := desc.Platform.OS, "haiku"; got != want {
		t.Fatalf("wrong OS; got: %v, want: %v", got, want)
	}
}

func TestDeduplicatedWrites(t *testing.T) {
	lp, err := FromPath(testPath)
	if err != nil {
		t.Fatalf("FromPath() = %v", err)
	}

	b, err := lp.Blob(configDigest)
	if err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer([]byte{})
	if _, err := io.Copy(buf, b); err != nil {
		log.Fatal(err)
	}

	if err := lp.WriteBlob(configDigest, io.NopCloser(bytes.NewBuffer(buf.Bytes()))); err != nil {
		t.Fatal(err)
	}

	if err := lp.WriteBlob(configDigest, io.NopCloser(bytes.NewBuffer(buf.Bytes()))); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveDescriptor(t *testing.T) {
	// need to set up a basic path
	tmp := t.TempDir()

	var ii v1.ImageIndex
	ii = empty.Index
	l, err := Write(tmp, ii)
	if err != nil {
		t.Fatal(err)
	}

	// add two images
	image1, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AppendImage(image1); err != nil {
		t.Fatal(err)
	}
	image2, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AppendImage(image2); err != nil {
		t.Fatal(err)
	}

	// remove one of the images by descriptor and ensure it is correct
	digest1, err := image1.Digest()
	if err != nil {
		t.Fatal(err)
	}
	digest2, err := image2.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.RemoveDescriptors(match.Digests(digest1)); err != nil {
		t.Fatal(err)
	}
	// ensure we only have one
	ii, err = l.ImageIndex()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ii.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Manifests) != 1 {
		t.Fatalf("mismatched manifests count, had %d, expected %d", len(manifest.Manifests), 1)
	}
	if manifest.Manifests[0].Digest != digest2 {
		t.Fatal("removed wrong digest")
	}
}

func TestReplaceIndex(t *testing.T) {
	// need to set up a basic path
	tmp := t.TempDir()

	var ii v1.ImageIndex
	ii = empty.Index
	l, err := Write(tmp, ii)
	if err != nil {
		t.Fatal(err)
	}

	// add two indexes
	index1, err := random.Index(1024, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AppendIndex(index1); err != nil {
		t.Fatal(err)
	}
	index2, err := random.Index(1024, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AppendIndex(index2); err != nil {
		t.Fatal(err)
	}
	index3, err := random.Index(1024, 3, 3)
	if err != nil {
		t.Fatal(err)
	}

	// remove one of the indexes by descriptor and ensure it is correct
	digest1, err := index1.Digest()
	if err != nil {
		t.Fatal(err)
	}
	digest3, err := index3.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.ReplaceIndex(index3, match.Digests(digest1)); err != nil {
		t.Fatal(err)
	}
	// ensure we only have one
	ii, err = l.ImageIndex()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ii.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Manifests) != 2 {
		t.Fatalf("mismatched manifests count, had %d, expected %d", len(manifest.Manifests), 2)
	}
	// we should have digest3, and *not* have digest1
	var have3 bool
	for _, m := range manifest.Manifests {
		if m.Digest == digest1 {
			t.Fatal("found digest1 still not replaced", digest1)
		}
		if m.Digest == digest3 {
			have3 = true
		}
	}
	if !have3 {
		t.Fatal("could not find digest3", digest3)
	}
}

func TestReplaceImage(t *testing.T) {
	// need to set up a basic path
	tmp := t.TempDir()

	var ii v1.ImageIndex
	ii = empty.Index
	l, err := Write(tmp, ii)
	if err != nil {
		t.Fatal(err)
	}

	// add two images
	image1, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AppendImage(image1); err != nil {
		t.Fatal(err)
	}
	image2, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AppendImage(image2); err != nil {
		t.Fatal(err)
	}
	image3, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}

	// remove one of the images by descriptor and ensure it is correct
	digest1, err := image1.Digest()
	if err != nil {
		t.Fatal(err)
	}
	digest3, err := image3.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.ReplaceImage(image3, match.Digests(digest1)); err != nil {
		t.Fatal(err)
	}
	// ensure we only have one
	ii, err = l.ImageIndex()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ii.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Manifests) != 2 {
		t.Fatalf("mismatched manifests count, had %d, expected %d", len(manifest.Manifests), 2)
	}
	// we should have digest3, and *not* have digest1
	var have3 bool
	for _, m := range manifest.Manifests {
		if m.Digest == digest1 {
			t.Fatal("found digest1 still not replaced", digest1)
		}
		if m.Digest == digest3 {
			have3 = true
		}
	}
	if !have3 {
		t.Fatal("could not find digest3", digest3)
	}
}

func TestRemoveBlob(t *testing.T) {
	// need to set up a basic path
	tmp := t.TempDir()

	var ii v1.ImageIndex = empty.Index
	l, err := Write(tmp, ii)
	if err != nil {
		t.Fatal(err)
	}

	// create a random blob
	b := []byte("abcdefghijklmnop")
	hash, _, err := v1.SHA256(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}

	if err := l.WriteBlob(hash, io.NopCloser(bytes.NewReader(b))); err != nil {
		t.Fatal(err)
	}
	// make sure it exists
	b2, err := l.Bytes(hash)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, b2) {
		t.Fatal("mismatched bytes")
	}
	// now the real test, delete it
	if err := l.RemoveBlob(hash); err != nil {
		t.Fatal(err)
	}
	// now it should not exist
	if _, err = l.Bytes(hash); err == nil {
		t.Fatal("still existed after deletion")
	}
}

func TestStreamingWriteLayer(t *testing.T) {
	// need to set up a basic path
	tmp := t.TempDir()

	var ii v1.ImageIndex = empty.Index
	l, err := Write(tmp, ii)
	if err != nil {
		t.Fatal(err)
	}

	// create a random streaming image and persist
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)
	go func() {
		pw.CloseWithError(func() error {
			body := "test file"
			if err := tw.WriteHeader(&tar.Header{
				Name:     "test.txt",
				Mode:     0600,
				Size:     int64(len(body)),
				Typeflag: tar.TypeReg,
			}); err != nil {
				return err
			}
			if _, err := tw.Write([]byte(body)); err != nil {
				return err
			}
			return tw.Close()
		}())
	}()
	img, err := mutate.Append(empty.Image, mutate.Addendum{
		Layer: stream.NewLayer(pr),
	})
	if err != nil {
		t.Fatalf("creating random streaming image failed: %v", err)
	}
	if _, err := img.Digest(); err == nil {
		t.Fatal("digesting image before stream is consumed; (v1.Image).Digest() = nil, expected err")
	}
	// AppendImage uses writeLayer
	if err := l.AppendImage(img); err != nil {
		t.Fatalf("(Path).AppendImage() = %v", err)
	}

	// Check that image was persisted and is valid
	imgDigest, err := img.Digest()
	if err != nil {
		t.Fatalf("(v1.Image).Digest() = %v", err)
	}
	img, err = l.Image(imgDigest)
	if err != nil {
		t.Fatalf("error loading image after writeLayer for validation; (Path).Image = %v", err)
	}
	if err := validate.Image(img); err != nil {
		t.Fatalf("validate.Image() = %v", err)
	}
}

func TestOverwriteWithWriteLayer(t *testing.T) {
	// need to set up a basic path
	tmp := t.TempDir()

	var ii v1.ImageIndex = empty.Index
	l, err := Write(tmp, ii)
	if err != nil {
		t.Fatal(err)
	}

	// create a random image and persist
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image() = %v", err)
	}
	imgDigest, err := img.Digest()
	if err != nil {
		t.Fatalf("(v1.Image).Digest() = %v", err)
	}
	if err := l.AppendImage(img); err != nil {
		t.Fatalf("(Path).AppendImage() = %v", err)
	}
	if err := validate.Image(img); err != nil {
		t.Fatalf("validate.Image() = %v", err)
	}

	// get the random image's layer
	layers, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(layers); n != 1 {
		t.Fatalf("expected image with 1 layer, got %d", n)
	}

	layer := layers[0]
	layerDigest, err := layer.Digest()
	if err != nil {
		t.Fatalf("(v1.Layer).Digest() = %v", err)
	}

	// truncate the layer contents on disk
	completeLayerBytes, err := l.Bytes(layerDigest)
	if err != nil {
		t.Fatalf("(Path).Bytes() = %v", err)
	}
	truncatedLayerBytes := completeLayerBytes[:512]

	path := l.path("blobs", layerDigest.Algorithm, layerDigest.Hex)
	if err := os.WriteFile(path, truncatedLayerBytes, os.ModePerm); err != nil {
		t.Fatalf("os.WriteFile(layerPath, truncated) = %v", err)
	}

	// ensure validation fails
	img, err = l.Image(imgDigest)
	if err != nil {
		t.Fatalf("error loading truncated image for validation; (Path).Image = %v", err)
	}
	if err := validate.Image(img); err == nil {
		t.Fatal("validating image after truncating layer; validate.Image() = nil, expected err")
	}

	// try writing expected contents with WriteBlob
	if err := l.WriteBlob(layerDigest, io.NopCloser(bytes.NewBuffer(completeLayerBytes))); err != nil {
		t.Fatalf("error attempting to overwrite truncated layer with valid layer; (Path).WriteBlob = %v", err)
	}

	// validation should still fail
	img, err = l.Image(imgDigest)
	if err != nil {
		t.Fatalf("error loading truncated image after WriteBlob for validation; (Path).Image = %v", err)
	}
	if err := validate.Image(img); err == nil {
		t.Fatal("validating image after attempting repair of truncated layer with WriteBlob; validate.Image() = nil, expected err")
	}

	// try writing expected contents with writeLayer
	if err := l.writeLayer(layer); err != nil {
		t.Fatalf("error attempting to overwrite truncated layer with valid layer; (Path).writeLayer = %v", err)
	}

	// validation should now succeed
	img, err = l.Image(imgDigest)
	if err != nil {
		t.Fatalf("error loading truncated image after writeLayer for validation; (Path).Image = %v", err)
	}
	if err := validate.Image(img); err != nil {
		t.Fatalf("validating image after attempting repair of truncated layer with writeLayer; validate.Image() = %v", err)
	}
}

func TestOverwriteWithReplaceImage(t *testing.T) {
	// need to set up a basic path
	tmp := t.TempDir()

	var ii v1.ImageIndex = empty.Index
	l, err := Write(tmp, ii)
	if err != nil {
		t.Fatal(err)
	}

	// create a random image and persist
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image() = %v", err)
	}
	imgDigest, err := img.Digest()
	if err != nil {
		t.Fatalf("(v1.Image).Digest() = %v", err)
	}
	if err := l.AppendImage(img); err != nil {
		t.Fatalf("(Path).AppendImage() = %v", err)
	}
	if err := validate.Image(img); err != nil {
		t.Fatalf("validate.Image() = %v", err)
	}

	// get the random image's layer
	layers, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(layers); n != 1 {
		t.Fatalf("expected image with 1 layer, got %d", n)
	}

	layer := layers[0]
	layerDigest, err := layer.Digest()
	if err != nil {
		t.Fatalf("(v1.Layer).Digest() = %v", err)
	}

	// truncate the layer contents on disk
	completeLayerBytes, err := l.Bytes(layerDigest)
	if err != nil {
		t.Fatalf("(Path).Bytes() = %v", err)
	}
	truncatedLayerBytes := completeLayerBytes[:512]

	path := l.path("blobs", layerDigest.Algorithm, layerDigest.Hex)
	if err := os.WriteFile(path, truncatedLayerBytes, os.ModePerm); err != nil {
		t.Fatalf("os.WriteFile(layerPath, truncated) = %v", err)
	}

	// ensure validation fails
	truncatedImg, err := l.Image(imgDigest)
	if err != nil {
		t.Fatalf("error loading truncated image for validation; (Path).Image = %v", err)
	}
	if err := validate.Image(truncatedImg); err == nil {
		t.Fatal("validating image after truncating layer; validate.Image() = nil, expected err")
	} else if strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("validating image after truncating layer; validate.Image() error is not helpful: %v", err)
	}

	// try writing expected contents with ReplaceImage
	if err := l.ReplaceImage(img, match.Digests(imgDigest)); err != nil {
		t.Fatalf("error attempting to overwrite truncated layer with valid layer; (Path).ReplaceImage = %v", err)
	}

	// validation should now succeed
	repairedImg, err := l.Image(imgDigest)
	if err != nil {
		t.Fatalf("error loading truncated image after ReplaceImage for validation; (Path).Image = %v", err)
	}
	if err := validate.Image(repairedImg); err != nil {
		t.Fatalf("validating image after attempting repair of truncated layer with ReplaceImage; validate.Image() = %v", err)
	}
}
