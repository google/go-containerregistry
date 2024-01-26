package local

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type pusher struct {
	path layout.Path
}

// Delete implements remote.Pusher.
func (lp *pusher) Delete(ctx context.Context, ref name.Reference) error {
	// TODO
	return errors.ErrUnsupported
}

// Push implements remote.Pusher.
func (lp *pusher) Push(ctx context.Context, ref name.Reference, t remote.Taggable) error {
	b, err := t.RawManifest()
	if err != nil {
		return err
	}
	h, sz, err := v1.SHA256(bytes.NewReader(b))
	if err != nil {
		return err
	}

	mt := types.DockerManifestSchema2

	if wmt, ok := t.(partial.WithMediaType); ok {
		m, err := wmt.MediaType()
		if err != nil {
			return err
		}
		mt = m
	}
	if err := lp.path.WriteBlob(h, io.NopCloser(bytes.NewBuffer(b))); err != nil {
		return err
	}

	return lp.path.AppendDescriptor(v1.Descriptor{
		MediaType: mt,
		Size:      sz,
		Digest:    h,
		Annotations: map[string]string{
			specsv1.AnnotationRefName: ref.Context().RepositoryStr(),
		},
	})
}

// Upload implements remote.Pusher.
func (lp *pusher) Upload(ctx context.Context, repo name.Repository, l v1.Layer) error {
	digest, err := l.Digest()
	if err != nil {
		return err
	}
	rc, err := l.Compressed()
	if err != nil {
		return err
	}
	return lp.path.WriteBlob(digest, rc)
}

func NewLocalPusher(path layout.Path) remote.Pusher {
	return &pusher{
		path,
	}
}

var _ remote.Pusher = (*pusher)(nil)
