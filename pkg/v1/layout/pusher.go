package layout

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type pusher struct {
	path Path
}

// Delete implements remote.Pusher.
func (lp *pusher) Delete(ctx context.Context, ref name.Reference) error {
	// TODO
	return errors.ErrUnsupported
}

func (lp *pusher) writeLayer(l v1.Layer) error {
	dg, err := l.Digest()
	if err != nil {
		return err
	}
	rc, err := l.Compressed()
	if err != nil {
		return err
	}
	err = lp.path.WriteBlob(dg, rc)
	if err != nil {
		return err
	}
	return nil
}

// Push implements remote.Pusher.
func (lp *pusher) Push(ctx context.Context, ref name.Reference, t remote.Taggable) error {
	mf, err := remote.TaggableToManifest(t)
	if err != nil {
		return err
	}
	b, desc, err := remote.UnpackTaggable(t)
	if err != nil {
		return err
	}
	if err := lp.path.WriteBlob(desc.Digest, io.NopCloser(bytes.NewBuffer(b))); err != nil {
		return err
	}

	if img, ok := mf.(v1.Image); ok {
		cl, err := partial.ConfigLayer(img)
		if err != nil {
			return err
		}
		dg, err := cl.Digest()
		if err != nil {
			return err
		}
		rc, err := img.RawConfigFile()
		if err != nil {
			return err
		}
		err = lp.path.WriteBlob(dg, io.NopCloser(bytes.NewBuffer(rc)))
		if err != nil {
			return err
		}
		ls, err := img.Layers()
		if err != nil {
			return err
		}
		for _, l := range ls {
			if err = lp.writeLayer(l); err != nil {
				return err
			}
		}
	}
	desc.Annotations = map[string]string{
		specsv1.AnnotationRefName: ref.String(),
	}
	return lp.path.AppendDescriptor(*desc)
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

func NewPusher(path Path) remote.Pusher {
	return &pusher{
		path,
	}
}

var _ remote.Pusher = (*pusher)(nil)
