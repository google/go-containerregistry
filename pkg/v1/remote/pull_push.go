package remote

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
)

type Puller interface {
	Layer(ctx context.Context, ref name.Digest) (v1.Layer, error)
	Head(ctx context.Context, ref name.Reference) (*v1.Descriptor, error)
	List(ctx context.Context, repo name.Repository) ([]string, error)
	// Deprecated: Use Artifact instead.
	// Get(ctx context.Context, ref name.Reference) (*Descriptor, error)
	Artifact(ctx context.Context, ref name.Reference) (partial.Artifact, error)
	Lister(ctx context.Context, repo name.Repository) (*Lister, error)
	Catalogger(ctx context.Context, reg name.Registry) (*Catalogger, error)
}
