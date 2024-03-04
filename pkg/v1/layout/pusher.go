// Copyright 2024 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package layout

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/types"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
)

func taggableToManifest(t partial.WithRawManifest) (partial.Artifact, error) {
	if a, ok := t.(partial.Artifact); ok {
		return a, nil
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
	return nil, fmt.Errorf("unknown taggable type")
}

func unpackTaggable(t partial.WithRawManifest) ([]byte, *v1.Descriptor, error) {
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

// Use partial.Artifact to unpack taggable.
// Duplication is not a concern here.
type pusher struct {
	path Path
}

// Delete implements remote.Pusher.
func (lp *pusher) Delete(_ context.Context, _ name.Reference) error {
	return errors.New("unsupported operation")
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

func (lp *pusher) writeLayers(pctx context.Context, img v1.Image) error {
	ls, err := img.Layers()
	if err != nil {
		return err
	}

	g, _ := errgroup.WithContext(pctx)

	for _, l := range ls {
		l := l

		g.Go(func() error {
			return lp.writeLayer(l)
		})
	}

	cl, err := partial.ConfigLayer(img)
	if errors.Is(err, stream.ErrNotComputed) {
		if err := g.Wait(); err != nil {
			return err
		}

		cl, err := partial.ConfigLayer(img)
		if err != nil {
			return err
		}

		return lp.writeLayer(cl)
	} else if err != nil {
		return err
	}

	g.Go(func() error {
		return lp.writeLayer(cl)
	})

	return g.Wait()
}

func (lp *pusher) writeChildren(pctx context.Context, idx v1.ImageIndex) error {
	children, err := partial.Manifests(idx)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(pctx)

	for _, child := range children {
		child := child
		if err := lp.writeChild(ctx, child, g); err != nil {
			return err
		}
	}

	return g.Wait()
}

func (lp *pusher) writeDeps(ctx context.Context, m partial.Artifact) error {
	if img, ok := m.(v1.Image); ok {
		return lp.writeLayers(ctx, img)
	}

	if idx, ok := m.(v1.ImageIndex); ok {
		return lp.writeChildren(ctx, idx)
	}

	// This has no deps, not an error (e.g. something you want to just PUT).
	return nil
}

func (lp *pusher) writeManifest(ctx context.Context, t partial.WithRawManifest) error {
	m, err := taggableToManifest(t)
	if err != nil {
		return err
	}

	needDeps := true

	if errors.Is(err, stream.ErrNotComputed) {
		if err := lp.writeDeps(ctx, m); err != nil {
			return err
		}
		needDeps = false
	} else if err != nil {
		return err
	}

	if needDeps {
		if err := lp.writeDeps(ctx, m); err != nil {
			return err
		}
	}

	b, desc, err := unpackTaggable(t)
	if err != nil {
		return err
	}
	if err := lp.path.WriteBlob(desc.Digest, io.NopCloser(bytes.NewBuffer(b))); err != nil {
		return err
	}

	return nil
}

func (lp *pusher) writeChild(ctx context.Context, child partial.Describable, g *errgroup.Group) error {
	switch child := child.(type) {
	case v1.ImageIndex:
		// For recursive index, we want to do a depth-first launching of goroutines
		// to avoid deadlocking.
		//
		// Note that this is rare, so the impact of this should be really small.
		return lp.writeManifest(ctx, child)
	case v1.Image:
		g.Go(func() error {
			return lp.writeManifest(ctx, child)
		})
	case v1.Layer:
		g.Go(func() error {
			return lp.writeLayer(child)
		})
	default:
		// This can't happen.
		return fmt.Errorf("encountered unknown child: %T", child)
	}
	return nil
}

// Push implements remote.Pusher.
func (lp *pusher) Push(ctx context.Context, ref name.Reference, t partial.WithRawManifest) error {
	err := lp.writeManifest(ctx, t)
	if err != nil {
		return err
	}
	_, desc, err := unpackTaggable(t)
	if err != nil {
		return err
	}
	desc.Annotations = map[string]string{
		specsv1.AnnotationRefName: ref.String(),
	}
	return lp.path.AppendDescriptor(*desc)
}

// Upload implements remote.Pusher.
func (lp *pusher) Upload(_ context.Context, _ name.Repository, l v1.Layer) error {
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
