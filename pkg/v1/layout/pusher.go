// Copyright 2019 The original author or authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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
	"github.com/google/go-containerregistry/pkg/v1/types"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
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

// Push implements remote.Pusher.
func (lp *pusher) Push(_ context.Context, ref name.Reference, t partial.WithRawManifest) error {
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
