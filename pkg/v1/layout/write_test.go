package layout

import (
	"io/ioutil"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/v1"
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

	original, err := Index(testPath)
	if err != nil {
		t.Fatal(err)
	}

	if layoutPath, err := Write(tmp, original); err != nil {
		t.Fatalf("Write(%s) = %v", tmp, err)
	} else if tmp != layoutPath.Path() {
		t.Fatalf("unexpected file system path %v", layoutPath)
	}

	written, err := Index(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if err := validate.Index(written); err != nil {
		t.Fatalf("validate.Index() = %v", err)
	}
}

func TestWriteErrors(t *testing.T) {
	idx, err := Index(testPath)
	if err != nil {
		t.Fatalf("Index() = %v", err)
	}
	img, err := Image(testPath, manifestDigest)
	if err != nil {
		t.Fatalf("Image() = %v", err)
	}

	// Found this here:
	// https://github.com/golang/go/issues/24195
	invalidPath := "double-null-padded-string\x00\x00"
	if _, err := Write(invalidPath, idx); err == nil {
		t.Fatalf("Write(%s) = nil, expected err", invalidPath)
	}
	if err := WriteIndex(invalidPath, idx); err == nil {
		t.Fatalf("WriteIndex(%s) = nil, expected err", invalidPath)
	}
	if err := WriteImage(invalidPath, img); err == nil {
		t.Fatalf("WriteIndex(%s) = nil, expected err", invalidPath)
	}
	if _, err := AppendIndex(invalidPath, idx); err == nil {
		t.Fatalf("WriteIndex(%s) = nil, expected err", invalidPath)
	}
	if _, err := AppendImage(invalidPath, img); err == nil {
		t.Fatalf("WriteIndex(%s) = nil, expected err", invalidPath)
	}
	if _, err := AppendDescriptor(invalidPath, v1.Descriptor{}); err == nil {
		t.Fatalf("WriteIndex(%s) = nil, expected err", invalidPath)
	}
}

func TestAppendDescriptorInitializesIndex(t *testing.T) {
	tmp, err := ioutil.TempDir("", "write-index-test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmp)

	// Append a descriptor to a non-existent layout.
	desc := v1.Descriptor{
		Digest:    bogusDigest,
		Size:      1337,
		MediaType: types.MediaType("not real"),
	}
	if _, err := AppendDescriptor(tmp, desc); err != nil {
		t.Fatalf("AppendDescriptor(%s) = %v", tmp, err)
	}

	// Read that layout from disk and make sure the descriptor is there.
	idx, err := Index(tmp)
	if err != nil {
		t.Fatalf("Index() = %v", err)
	}
	manifest, err := idx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest() = %v", err)
	}
	if diff := cmp.Diff(manifest.Manifests[0], desc); diff != "" {
		t.Fatalf("bad descriptor: (-got +want) %s", diff)
	}
}

func TestAppendArtifacts(t *testing.T) {
	tmp, err := ioutil.TempDir("", "write-index-test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmp)

	original, err := Index(testPath)
	if err != nil {
		t.Fatal(err)
	}
	originalManifest, err := original.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}

	// Let's reconstruct the original.
	for i, desc := range originalManifest.Manifests {
		// Each descriptor is annotated with its position.
		annotations := map[string]string{
			"org.opencontainers.image.ref.name": strconv.Itoa(i + 1),
		}
		switch desc.MediaType {
		case types.OCIImageIndex, types.DockerManifestList:
			ii, err := original.ImageIndex(desc.Digest)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := AppendIndex(tmp, ii, WithAnnotations(annotations)); err != nil {
				t.Fatal(err)
			}
		case types.OCIManifestSchema1, types.DockerManifestSchema2:
			img, err := original.Image(desc.Digest)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := AppendImage(tmp, img, WithAnnotations(annotations)); err != nil {
				t.Fatal(err)
			}
		}
	}

	reconstructed, err := Index(tmp)
	if err != nil {
		t.Fatalf("Index() = %v", err)
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
	options := []LayoutOption{
		WithAnnotations(annotations),
		WithURLs(urls),
		WithPlatform(platform),
	}
	idx, err := AppendImage(tmp, img, options...)
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
