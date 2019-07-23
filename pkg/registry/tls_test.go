package registry_test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestTLS(t *testing.T) {
	s, tp, err := registry.TLS("test.com")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	i, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("Unable to make image: %v", err)
	}
	rd, err := i.Digest()
	if err != nil {
		t.Fatalf("Unable to get image digest: %v", err)
	}

	d, err := name.NewDigest("test.com/foo@" + rd.String())
	if err != nil {
		t.Fatalf("Unable to parse digest: %v", err)
	}
	if err := remote.Write(d, i, remote.WithTransport(tp)); err != nil {
		t.Fatalf("Unable to write image to remove: %s", err)
	}
}
