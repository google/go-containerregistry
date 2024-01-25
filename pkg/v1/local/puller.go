package local

import (
	"context"
	"errors"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type puller struct {
}

var _ remote.Puller = (*puller)(nil)

// Artifact implements remote.Puller.
func (*puller) Artifact(ctx context.Context, ref name.Reference) (partial.Artifact, error) {
	panic("unimplemented")
}

// Get implements remote.Puller.
func (*puller) Get(ctx context.Context, ref name.Reference) (*remote.Descriptor, error) {
	panic("unimplemented")
}

// Head implements remote.Puller.
func (*puller) Head(ctx context.Context, ref name.Reference) (*v1.Descriptor, error) {
	panic("unimplemented")
}

// Layer implements remote.Puller.
func (*puller) Layer(ctx context.Context, ref name.Digest) (v1.Layer, error) {
	panic("unimplemented")
}

// List implements remote.Puller.
func (*puller) List(ctx context.Context, repo name.Repository) ([]string, error) {
	return nil, errors.New("unimplemented")
}

// Lister implements remote.Puller.
func (*puller) Lister(ctx context.Context, repo name.Repository) (*remote.Lister, error) {
	return nil, errors.New("unimplemented")
}

// Catalogger implements remote.Puller.
func (*puller) Catalogger(ctx context.Context, reg name.Registry) (*remote.Catalogger, error) {
	return nil, errors.New("unimplemented")
}
