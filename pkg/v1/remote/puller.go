// Copyright 2023 Google LLC All Rights Reserved.
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

package remote

import (
	"context"
	"errors"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type Puller interface {
	Layer(ctx context.Context, ref name.Digest) (v1.Layer, error)
	Head(ctx context.Context, ref name.Reference) (*v1.Descriptor, error)
	List(ctx context.Context, repo name.Repository) ([]string, error)
	// Deprecated: Use Artifact instead.
	Get(ctx context.Context, ref name.Reference) (*Descriptor, error)
	Artifact(ctx context.Context, ref name.Reference) (partial.Artifact, error)
	Lister(ctx context.Context, repo name.Repository) (*Lister, error)
	Catalogger(ctx context.Context, reg name.Registry) (*Catalogger, error)
	Catalog(ctx context.Context, reg name.Registry) ([]string, error)
	Referrers(ctx context.Context, d name.Digest, filter map[string]string) (v1.ImageIndex, error)
}

type puller struct {
	o *options

	// map[resource]*reader
	readers sync.Map
}

var _ Puller = (*puller)(nil)

func NewPuller(options ...Option) (Puller, error) {
	o, err := makeOptions(options...)
	if err != nil {
		return nil, err
	}
	return newPuller(o), nil
}

func newPuller(o *options) Puller {
	if o.puller != nil {
		return *o.puller
	}
	return &puller{
		o: o,
	}
}

type reader struct {
	// in
	target resource
	o      *options

	// f()
	once sync.Once

	// out
	f   *fetcher
	err error
}

// this will run once per reader instance
func (r *reader) init(ctx context.Context) error {
	r.once.Do(func() {
		r.f, r.err = makeFetcher(ctx, r.target, r.o)
	})
	return r.err
}

func (p *puller) fetcher(ctx context.Context, target resource) (*fetcher, error) {
	v, _ := p.readers.LoadOrStore(target, &reader{
		target: target,
		o:      p.o,
	})
	rr := v.(*reader)
	return rr.f, rr.init(ctx)
}

// Head is like remote.Head, but avoids re-authenticating when possible.
func (p *puller) Head(ctx context.Context, ref name.Reference) (*v1.Descriptor, error) {
	f, err := p.fetcher(ctx, ref.Context())
	if err != nil {
		return nil, err
	}

	return f.headManifest(ctx, ref, allManifestMediaTypes)
}

// Get is like remote.Get, but avoids re-authenticating when possible.
func (p *puller) Get(ctx context.Context, ref name.Reference) (*Descriptor, error) {
	return p.get(ctx, ref, allManifestMediaTypes, p.o.platform)
}

func (p *puller) get(ctx context.Context, ref name.Reference, acceptable []types.MediaType, platform v1.Platform) (*Descriptor, error) {
	f, err := p.fetcher(ctx, ref.Context())
	if err != nil {
		return nil, err
	}
	mf, desc, err := f.fetchManifest(ctx, ref, acceptable)
	if err != nil {
		return nil, err
	}
	return &Descriptor{
		ref:        ref,
		ctx:        ctx,
		fetcher:    *f,
		Manifest:   mf,
		Descriptor: *desc,
		platform:   platform,
	}, nil
}

func (p *puller) Artifact(ctx context.Context, ref name.Reference) (partial.Artifact, error) {
	return p.artifact(ctx, ref, p.o.acceptableMediaTypes, p.o.platform)
}

func (p *puller) artifact(ctx context.Context, ref name.Reference, acceptable []types.MediaType, platform v1.Platform) (partial.Artifact, error) {
	desc, err := p.get(ctx, ref, acceptable, platform)
	if err != nil {
		return nil, err
	}
	if desc.MediaType.IsImage() {
		return desc.Image()
	} else if desc.MediaType.IsIndex() {
		return desc.ImageIndex()
	} else if desc.MediaType.IsSchema1() {
		return desc.Schema1()
	}
	return nil, errors.New("TODO: ???")
}

// Layer is like remote.Layer, but avoids re-authenticating when possible.
func (p *puller) Layer(ctx context.Context, ref name.Digest) (v1.Layer, error) {
	f, err := p.fetcher(ctx, ref.Context())
	if err != nil {
		return nil, err
	}

	h, err := v1.NewHash(ref.Identifier())
	if err != nil {
		return nil, err
	}
	l, err := partial.CompressedToLayer(&remoteLayer{
		fetcher: *f,
		ctx:     ctx,
		digest:  h,
	})
	if err != nil {
		return nil, err
	}
	return &MountableLayer{
		Layer:     l,
		Reference: ref,
	}, nil
}

// List lists tags in a repo and handles pagination, returning the full list of tags.
func (p *puller) List(ctx context.Context, repo name.Repository) ([]string, error) {
	lister, err := p.Lister(ctx, repo)
	if err != nil {
		return nil, err
	}

	tagList := []string{}
	for lister.HasNext() {
		tags, err := lister.Next(ctx)
		if err != nil {
			return nil, err
		}
		tagList = append(tagList, tags.Tags...)
	}

	return tagList, nil
}

// Lister lists tags in a repo and returns a Lister for paginating through the results.
func (p *puller) Lister(ctx context.Context, repo name.Repository) (*Lister, error) {
	return p.lister(ctx, repo, p.o.pageSize)
}

func (p *puller) lister(ctx context.Context, repo name.Repository, pageSize int) (*Lister, error) {
	f, err := p.fetcher(ctx, repo)
	if err != nil {
		return nil, err
	}
	page, err := f.listPage(ctx, repo, "", pageSize)
	if err != nil {
		return nil, err
	}
	return &Lister{
		f:        f,
		repo:     repo,
		pageSize: pageSize,
		page:     page,
		err:      err,
	}, nil
}

// Catalog lists repos in a registry and handles pagination, returning the full list of repos.
func (p *puller) Catalog(ctx context.Context, reg name.Registry) ([]string, error) {
	return p.catalog(ctx, reg, p.o.pageSize)
}

func (p *puller) catalog(ctx context.Context, reg name.Registry, pageSize int) ([]string, error) {
	catalogger, err := p.catalogger(ctx, reg, pageSize)
	if err != nil {
		return nil, err
	}
	repoList := []string{}
	for catalogger.HasNext() {
		repos, err := catalogger.Next(ctx)
		if err != nil {
			return nil, err
		}
		repoList = append(repoList, repos.Repos...)
	}
	return repoList, nil
}

// Catalogger lists repos in a registry and returns a Catalogger for paginating through the results.
func (p *puller) Catalogger(ctx context.Context, reg name.Registry) (*Catalogger, error) {
	return p.catalogger(ctx, reg, p.o.pageSize)
}

func (p *puller) catalogger(ctx context.Context, reg name.Registry, pageSize int) (*Catalogger, error) {
	f, err := p.fetcher(ctx, reg)
	if err != nil {
		return nil, err
	}
	page, err := f.catalogPage(ctx, reg, "", pageSize)
	if err != nil {
		return nil, err
	}
	return &Catalogger{
		f:        f,
		reg:      reg,
		pageSize: pageSize,
		page:     page,
		err:      err,
	}, nil
}

func (p *puller) Referrers(ctx context.Context, d name.Digest, filter map[string]string) (v1.ImageIndex, error) {
	f, err := p.fetcher(ctx, d.Context())
	if err != nil {
		return nil, err
	}
	return f.fetchReferrers(ctx, filter, d)
}
