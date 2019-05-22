// Package cache provides methods to cache layers.
package cache

import (
	"errors"

	"github.com/google/go-containerregistry/pkg/v1"
)

// Cache encapsulates methods to interact with cached layers.
type Cache interface {
	// Put writes the Layer to the Cache.
	//
	// The returned Layer should be used for future operations, since lazy
	// cachers might only populate the cache when the layer is actually
	// consumed.
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
// underlyiing Image.
func NewImage(i v1.Image, c Cache) v1.Image {
	return &image{
		Image: i,
		c:     c,
	}
}

type image struct {
	v1.Image
	c Cache
}

func (i *image) LayerByDigest(h v1.Hash) (v1.Layer, error) {
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
