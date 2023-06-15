package registry_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestDiskPush(t *testing.T) {
	dir := t.TempDir()
	reg := registry.New(registry.WithBlobHandler(registry.NewDiskBlobHandler(dir)))
	srv := httptest.NewServer(reg)
	defer srv.Close()

	ref, err := name.ParseReference(strings.TrimPrefix(srv.URL, "http://") + "/foo/bar:latest")
	if err != nil {
		t.Fatal(err)
	}
	img, err := random.Image(1024, 5)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatalf("remote.Write: %v", err)
	}

	// Test we can read and validate the image.
	if _, err := remote.Image(ref); err != nil {
		t.Fatalf("remote.Image: %v", err)
	}
	if err := validate.Image(img); err != nil {
		t.Fatalf("validate.Image: %v", err)
	}

	// Collect the layer SHAs we expect to find.
	want := map[string]bool{}
	if h, err := img.ConfigName(); err != nil {
		t.Fatal(err)
	} else {
		want[h.String()] = true
	}
	ls, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range ls {
		if h, err := l.Digest(); err != nil {
			t.Fatal(err)
		} else {
			want[h.String()] = true
		}
	}

	// Test the blobs are there on disk.
	for dig := range want {
		if _, err := os.Stat(filepath.Join(dir, dig)); err != nil {
			t.Fatalf("os.Stat(%s): %v", dig, err)
		}
		t.Logf("Found %s", dig)
	}
}
