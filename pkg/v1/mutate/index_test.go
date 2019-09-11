package mutate_test

import (
	"log"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestAppendIndex(t *testing.T) {
	base, err := random.Index(1024, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := random.Index(2048, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	img, err := random.Image(4096, 5)
	if err != nil {
		t.Fatal(err)
	}
	l, err := random.Layer(1024, types.OCIUncompressedRestrictedLayer)
	if err != nil {
		t.Fatal(err)
	}

	add := mutate.AppendManifests(base, mutate.IndexAddendum{
		Add: idx,
		Descriptor: v1.Descriptor{
			URLs: []string{"index.example.com"},
		},
	}, mutate.IndexAddendum{
		Add: img,
		Descriptor: v1.Descriptor{
			URLs: []string{"image.example.com"},
		},
	}, mutate.IndexAddendum{
		Add: l,
		Descriptor: v1.Descriptor{
			URLs: []string{"layer.example.com"},
		},
	})

	if err := validate.Index(add); err != nil {
		t.Errorf("Validate() = %v", err)
	}

	got, err := add.MediaType()
	if err != nil {
		t.Fatal(err)
	}
	want, err := base.MediaType()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("MediaType() = %s != %s", got, want)
	}

	// TODO(jonjohnsonjr): There's no way to grab layers from v1.ImageIndex.
	m, err := add.IndexManifest()
	if err != nil {
		log.Fatal(err)
	}

	for i, want := range map[int]string{
		3: "index.example.com",
		4: "image.example.com",
		5: "layer.example.com",
	} {
		if got := m.Manifests[i].URLs[0]; got != want {
			t.Errorf("wrong URLs[0] for Manifests[%d]: %s != %s", i, got, want)
		}
	}

	if got, want := m.Manifests[5].MediaType, types.OCIUncompressedRestrictedLayer; got != want {
		t.Errorf("wrong MediaType for layer: %s != %s", got, want)
	}
}
