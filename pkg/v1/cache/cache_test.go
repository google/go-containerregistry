package cache

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

func TestCache(t *testing.T) {
	var numLayers int64 = 5
	img, err := random.Image(10, numLayers)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	m := &memcache{map[v1.Hash]v1.Layer{}}
	img = NewImage(img, m)

	// Cache is empty.
	if len(m.m) != 0 {
		t.Errorf("Before consuming, cache is non-empty: %+v", m.m)
	}

	// Consume each layer, cache gets populated.
	ls, err := img.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}
	for i, l := range ls {
		h, err := l.Digest()
		if err != nil {
			t.Fatalf("layer.Digest: %v", err)
		}
		if _, err := img.LayerByDigest(h); err != nil {
			t.Fatalf("LayerByDigest: %v", err)
		}
		if got, want := len(m.m), i+1; got != want {
			t.Errorf("Cache has %d entries, want %d", got, want)
		}
	}
}

// TODO: test that caching short-circuits LayerByDigest, the underlying image
// won't be called twice.

// TODO: test FS impl writes files that it can also read.

type memcache struct {
	m map[v1.Hash]v1.Layer
}

func (m *memcache) Put(l v1.Layer) error {
	h, err := l.Digest()
	if err != nil {
		return err
	}
	m.m[h] = l
	return nil
}

func (m *memcache) Get(h v1.Hash) (v1.Layer, error) {
	l, found := m.m[h]
	if !found {
		return nil, ErrNotFound
	}
	return l, nil
}

func (m *memcache) Delete(h v1.Hash) error {
	delete(m.m, h)
	return nil
}
