// Package cache provides methods to cache layers.
package cache

import (
	"errors"
	"log"

	"github.com/google/go-containerregistry/pkg/v1"
)

// Cache encapsulates methods to interact with cached layers.
type Cache interface {
	// Put writes the Layer to the Cache.
	//
	// The returned Layer should be used for future operations, since lazy
	// cachers might only populate the cache when the layer is actually
	// consumed.
	//
	// The returned layer can be consumed, and the cache entry populated,
	// by calling either Compressed or Uncompressed and consuming the
	// returned io.ReadCloser.
	Put(v1.Layer) (v1.Layer, error)

	// Get returns the Layer cached by the given Hash, or ErrNotFound if no
	// such layer was found.
	Get(v1.Hash) (v1.Layer, error)

	// Delete removes the Layer with the given Hash from the Cache.
	Delete(v1.Hash) error
}

// ErrNotFound is returned by Get when no layer with the given Hash is found.
var ErrNotFound = errors.New("layer was not found")

// NewImage returns a new Image whose layers will be pulled from the cache if
// they are found, and written to the cache as they are read from the
// underlying Image.
func NewImage(i v1.Image, c Cache) *Image {
	return &Image{
		Image: i,
		c:     c,
	}
}

type Image struct {
	v1.Image
	c Cache
}

func (i *Image) Layers() ([]v1.Layer, error) {
	ls, err := i.Image.Layers()
	if err != nil {
		return nil, err
	}

	var out []v1.Layer
	for _, l := range ls {
		// Check if this layer is present in the cache in compressed
		// form.
		digest, err := l.Digest()
		if err != nil {
			return nil, err
		}
		if cl, err := i.c.Get(digest); err == nil {
			// Layer found in the cache.
			log.Printf("Layer %s found (compressed) in cache", digest)
			out = append(out, cl)
			continue
		} else if err != nil && err != ErrNotFound {
			return nil, err
		}

		// Check if this layer is present in the cache in
		// uncompressed form.
		diffID, err := l.DiffID()
		if err != nil {
			return nil, err
		}
		if cl, err := i.c.Get(diffID); err == nil {
			// Layer found in the cache.
			log.Printf("Layer %s found (uncompressed) in cache", diffID)
			out = append(out, cl)
		} else if err != nil && err != ErrNotFound {
			return nil, err
		}

		// Not cached, fall through to real layer.
		l, err = i.c.Put(l)
		if err != nil {
			return nil, err
		}
		out = append(out, l)

	}
	return out, nil
}

func (i *Image) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	l, err := i.c.Get(h)
	if err == ErrNotFound {
		// Not cached, get it and write it.
		l, err := i.Image.LayerByDigest(h)
		if err != nil {
			return nil, err
		}
		return i.c.Put(l)
	}
	return l, err
}

func (i *Image) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	l, err := i.c.Get(h)
	if err == ErrNotFound {
		// Not cached, get it and write it.
		l, err := i.Image.LayerByDiffID(h)
		if err != nil {
			return nil, err
		}
		return i.c.Put(l)
	}
	return l, err
}
