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
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type Puller struct {
	o *options

	// map[resource]*reader
	readers sync.Map
}

func NewPuller(options ...Option) (*Puller, error) {
	o, err := makeOptions(options...)
	if err != nil {
		return nil, err
	}

	return newPuller(o), nil
}

func newPuller(o *options) *Puller {
	return &Puller{
		o: o,
	}
}

type reader struct {
	target resource
	o      *options
	once   sync.Once

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

func (p *Puller) reader(ctx context.Context, target resource) (*reader, error) {
	v, _ := p.readers.LoadOrStore(target, &reader{
		target: target,
		o:      p.o,
	})
	rr := v.(*reader)
	return rr, rr.init(ctx)
}

// Head is like remote.Head, but avoids re-authenticating when possible.
func (p *Puller) Head(ctx context.Context, ref name.Reference) (*v1.Descriptor, error) {
	r, err := p.reader(ctx, ref.Context())
	if err != nil {
		return nil, err
	}

	return r.f.headManifest(ctx, ref, allManifestMediaTypes)
}

// Get is like remote.Get, but avoids re-authenticating when possible.
func (p *Puller) Get(ctx context.Context, ref name.Reference) (*Descriptor, error) {
	r, err := p.reader(ctx, ref.Context())
	if err != nil {
		return nil, err
	}
	return r.f.get(ctx, ref, allManifestMediaTypes)
}

// Layer is like remote.Layer, but avoids re-authenticated when possible.
func (p *Puller) Layer(ctx context.Context, ref name.Digest) (v1.Layer, error) {
	r, err := p.reader(ctx, ref.Context())
	if err != nil {
		return nil, err
	}

	h, err := v1.NewHash(ref.Identifier())
	if err != nil {
		return nil, err
	}
	l, err := partial.CompressedToLayer(&remoteLayer{
		fetcher: *r.f,
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
func (p *Puller) List(ctx context.Context, repo name.Repository) ([]string, error) {
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
func (p *Puller) Lister(ctx context.Context, repo name.Repository) (*Lister, error) {
	r, err := p.reader(ctx, repo)
	if err != nil {
		return nil, err
	}
	page, err := r.f.listPage(ctx, repo, "")
	if err != nil {
		return nil, err
	}
	return &Lister{
		f:    r.f,
		repo: repo,
		page: page,
		err:  err,
	}, nil
}

// Catalog lists repos in a registry and handles pagination, returning the full list of repos.
func (p *Puller) Catalog(ctx context.Context, reg name.Registry) ([]string, error) {
	catalogger, err := p.Catalogger(ctx, reg)
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
func (p *Puller) Catalogger(ctx context.Context, reg name.Registry) (*Catalogger, error) {
	r, err := p.reader(ctx, reg)
	if err != nil {
		return nil, err
	}
	page, err := r.f.catalogPage(ctx, reg, "")
	if err != nil {
		return nil, err
	}
	return &Catalogger{
		f:    r.f,
		reg:  reg,
		page: page,
		err:  err,
	}, nil
}

func (p *Puller) get(ctx context.Context, ref name.Reference, acceptable []types.MediaType) (*Descriptor, error) {
	r, err := p.reader(ctx, ref.Context())
	if err != nil {
		return nil, err
	}
	return r.f.get(ctx, ref, acceptable)
}

func (p *Puller) referrers(ctx context.Context, d name.Digest, filter map[string]string) (v1.ImageIndex, error) {
	r, err := p.reader(ctx, d.Context())
	if err != nil {
		return nil, err
	}
	return r.f.fetchReferrers(ctx, filter, d)
}
