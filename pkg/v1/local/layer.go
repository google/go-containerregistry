package local

import (
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// remoteImagelayer implements partial.CompressedLayer
type localLayer struct {
	path   layout.Path
	digest v1.Hash
}

// Compressed implements partial.CompressedLayer.
func (lp *localLayer) Compressed() (io.ReadCloser, error) {
	return lp.path.Blob(lp.digest)
}

// Digest implements partial.CompressedLayer.
func (lp *localLayer) Digest() (v1.Hash, error) {
	return lp.digest, nil
}

// MediaType implements partial.CompressedLayer.
func (*localLayer) MediaType() (types.MediaType, error) {
	// TODO
	return types.DockerLayer, nil
}

// Size implements partial.CompressedLayer.
func (rl *localLayer) Size() (int64, error) {
	return rl.path.BlobSize(rl.digest)
}

// See partial.Exists.
func (rl *localLayer) Exists() (bool, error) {
	return rl.path.BlobExists(rl.digest), nil
}

var _ partial.CompressedLayer = (*localLayer)(nil)
