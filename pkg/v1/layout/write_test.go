package layout

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestWrite(t *testing.T) {
	tmp, err := ioutil.TempDir("", "write-index-test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmp)

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
	tmp, err := ioutil.TempDir("", "write-index-test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmp)
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
	tmp, err := ioutil.TempDir("", "write-index-test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmp)

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
	tmp, err := ioutil.TempDir("", "write-index-test")
	if err != nil {
		t.Fatal(err)
	}
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

	if err := lp.WriteBlob(configDigest, ioutil.NopCloser(bytes.NewBuffer(buf.Bytes()))); err != nil {
		t.Fatal(err)
	}

	if err := lp.WriteBlob(configDigest, ioutil.NopCloser(bytes.NewBuffer(buf.Bytes()))); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveDescriptor(t *testing.T) {
	// need to set up a basic path
	tmp, err := ioutil.TempDir("", "remove-descriptor-test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmp)

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
	tmp, err := ioutil.TempDir("", "replace-index-test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmp)

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
	tmp, err := ioutil.TempDir("", "replace-image-test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmp)

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
	tmp, err := ioutil.TempDir("", "remove-blob-test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmp)

	var ii v1.ImageIndex
	ii = empty.Index
	l, err := Write(tmp, ii)
	if err != nil {
		t.Fatal(err)
	}

	// create a random blob
	b := []byte("abcdefghijklmnop")
	hash, _, err := v1.SHA256(bytes.NewReader(b))

	if err := l.WriteBlob(hash, ioutil.NopCloser(bytes.NewReader(b))); err != nil {
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
	b2, err = l.Bytes(hash)
	if err == nil {
		t.Fatal("still existed after deletion")
	}
}
