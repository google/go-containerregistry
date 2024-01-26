package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type manifest interface {
	remote.Taggable
	partial.Describable
}

type describable struct {
	desc v1.Descriptor
}

func (d describable) Digest() (v1.Hash, error) {
	return d.desc.Digest, nil
}

func (d describable) Size() (int64, error) {
	return d.desc.Size, nil
}

func (d describable) MediaType() (types.MediaType, error) {
	return d.desc.MediaType, nil
}

type tagManifest struct {
	remote.Taggable
	partial.Describable
}

func taggableToManifest(t remote.Taggable) (manifest, error) {
	if m, ok := t.(manifest); ok {
		return m, nil
	}

	if d, ok := t.(*remote.Descriptor); ok {
		if d.MediaType.IsIndex() {
			return d.ImageIndex()
		}

		if d.MediaType.IsImage() {
			return d.Image()
		}

		if d.MediaType.IsSchema1() {
			return d.Schema1()
		}

		return tagManifest{t, describable{d.ToDescriptor()}}, nil
	}

	desc := v1.Descriptor{
		// A reasonable default if Taggable doesn't implement MediaType.
		MediaType: types.DockerManifestSchema2,
	}

	b, err := t.RawManifest()
	if err != nil {
		return nil, err
	}

	if wmt, ok := t.(partial.WithMediaType); ok {
		desc.MediaType, err = wmt.MediaType()
		if err != nil {
			return nil, err
		}
	}

	desc.Digest, desc.Size, err = v1.SHA256(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	return tagManifest{t, describable{desc}}, nil
}

func unpackTaggable(t remote.Taggable) ([]byte, *v1.Descriptor, error) {
	if d, ok := t.(*remote.Descriptor); ok {
		return d.Manifest, &d.Descriptor, nil
	}
	b, err := t.RawManifest()
	if err != nil {
		return nil, nil, err
	}

	// A reasonable default if Taggable doesn't implement MediaType.
	mt := types.DockerManifestSchema2

	if wmt, ok := t.(partial.WithMediaType); ok {
		m, err := wmt.MediaType()
		if err != nil {
			return nil, nil, err
		}
		mt = m
	}

	h, sz, err := v1.SHA256(bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}

	return b, &v1.Descriptor{
		MediaType: mt,
		Size:      sz,
		Digest:    h,
	}, nil
}

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
	mf, err := taggableToManifest(t)
	if err != nil {
		return err
	}
	b, desc, err := unpackTaggable(t)
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
		lp.path.WriteBlob(dg, io.NopCloser(bytes.NewBuffer(rc)))
		fmt.Fprintln(os.Stderr, "image!")
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
	fmt.Fprintln(os.Stderr, digest)
	return lp.path.WriteBlob(digest, rc)
}

func NewPusher(path layout.Path) remote.Pusher {
	return &pusher{
		path,
	}
}

var _ remote.Pusher = (*pusher)(nil)
