package local

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type puller struct {
	path layout.Path
}

func NewLocalPuller(path layout.Path) remote.Puller {
	return &puller{
		path,
	}
}

var _ remote.Puller = (*puller)(nil)

// Artifact implements remote.Puller.
func (p *puller) getDescriptor(ctx context.Context, ref name.Reference) (*v1.Descriptor, error) {
	idx, err := p.path.ImageIndex()
	if err != nil {
		return nil, err
	}
	im, err := idx.IndexManifest()
	if err != nil {
		return nil, err
	}
	for _, manifest := range im.Manifests {
		if rref, ok := manifest.Annotations[specsv1.AnnotationRefName]; ok {
			if ref.String() == rref {
				return &manifest, nil
			}
		}
	}
	return nil, fmt.Errorf("unknown image: %s", ref.String())
}

// Artifact implements remote.Puller.
func (p *puller) Artifact(ctx context.Context, ref name.Reference) (partial.Artifact, error) {
	desc, err := p.getDescriptor(ctx, ref)
	if err != nil {
		return nil, err
	}
	if desc.MediaType.IsImage() {
		return p.path.Image(desc.Digest)
	} else if desc.MediaType.IsIndex() {
		reg, err := p.path.ImageIndex()
		if err != nil {
			return nil, err
		}
		return reg.ImageIndex(desc.Digest)
	}
	return nil, errors.ErrUnsupported
}

// Head implements remote.Puller.
func (p *puller) Head(ctx context.Context, ref name.Reference) (*v1.Descriptor, error) {
	return p.getDescriptor(ctx, ref)
}

// Layer implements remote.Puller.
func (p *puller) Layer(ctx context.Context, ref name.Digest) (v1.Layer, error) {
	h, err := v1.NewHash(ref.Identifier())
	if err != nil {
		return nil, err
	}
	l, err := partial.CompressedToLayer(&localLayer{
		path:   p.path,
		digest: h,
	})
	if err != nil {
		return nil, err
	}
	return l, nil
}

// List implements remote.Puller.
func (*puller) List(ctx context.Context, repo name.Repository) ([]string, error) {
	return nil, errors.ErrUnsupported
}

// Get implements remote.Puller.
func (*puller) Get(ctx context.Context, ref name.Reference) (*remote.Descriptor, error) {
	return nil, errors.ErrUnsupported
}

// Lister implements remote.Puller.
func (*puller) Lister(ctx context.Context, repo name.Repository) (*remote.Lister, error) {
	return nil, errors.ErrUnsupported
}

// Catalogger implements remote.Puller.
func (*puller) Catalogger(ctx context.Context, reg name.Registry) (*remote.Catalogger, error) {
	return nil, errors.ErrUnsupported
}
