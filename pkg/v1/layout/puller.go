// Copyright 2019 The original author or authors
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
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type puller struct {
	path Path
}

func NewPuller(path Path) remote.Puller {
	return &puller{
		path,
	}
}

var _ remote.Puller = (*puller)(nil)

// Artifact implements remote.Puller.
func (p *puller) getDescriptor(ref name.Reference) (*v1.Descriptor, error) {
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
func (p *puller) Artifact(_ context.Context, ref name.Reference) (partial.Artifact, error) {
	desc, err := p.getDescriptor(ref)
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
	} else if desc.MediaType.IsSchema1() {
		return nil, fmt.Errorf("layout puller does not support schema1 images")
	}
	return nil, fmt.Errorf("unknown media type: %s", desc.MediaType)
}

// Head implements remote.Puller.
func (p *puller) Head(_ context.Context, ref name.Reference) (*v1.Descriptor, error) {
	return p.getDescriptor(ref)
}

// Layer implements remote.Puller.
func (p *puller) Layer(_ context.Context, ref name.Digest) (v1.Layer, error) {
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
func (*puller) List(_ context.Context, _ name.Repository) ([]string, error) {
	return nil, fmt.Errorf("unsupported operation")
}

// Get implements remote.Puller.
func (*puller) Get(_ context.Context, _ name.Reference) (*remote.Descriptor, error) {
	return nil, fmt.Errorf("unsupported operation")
}

// Lister implements remote.Puller.
func (*puller) Lister(_ context.Context, _ name.Repository) (*remote.Lister, error) {
	return nil, fmt.Errorf("unsupported operation")
}

// Catalogger implements remote.Puller.
func (*puller) Catalogger(_ context.Context, _ name.Registry) (*remote.Catalogger, error) {
	return nil, fmt.Errorf("unsupported operation")
}

// Catalog implements remote.Puller.
func (*puller) Catalog(_ context.Context, _ name.Registry) ([]string, error) {
	return nil, fmt.Errorf("unsupported operation")
}

// Referrers implements remote.Puller.
func (*puller) Referrers(_ context.Context, _ name.Digest, _ map[string]string) (v1.ImageIndex, error) {
	return nil, fmt.Errorf("unsupported operation")
}
