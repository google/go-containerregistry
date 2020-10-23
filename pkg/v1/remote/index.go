// Copyright 2018 Google LLC All Rights Reserved.
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
	"fmt"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

var acceptableIndexMediaTypes = []types.MediaType{
	types.DockerManifestList,
	types.OCIImageIndex,
}

// remoteIndex accesses an index from a remote registry
type remoteIndex struct {
	fetcher
	manifestLock sync.Mutex // Protects manifest
	manifest     []byte
	mediaType    types.MediaType
	descriptor   *v1.Descriptor
}

// Index provides access to a remote index reference.
func Index(ref name.Reference, options ...Option) (v1.ImageIndex, error) {
	desc, err := get(ref, acceptableIndexMediaTypes, options...)
	if err != nil {
		return nil, err
	}

	return desc.ImageIndex()
}

func (r *remoteIndex) MediaType() (types.MediaType, error) {
	if string(r.mediaType) != "" {
		return r.mediaType, nil
	}
	return types.DockerManifestList, nil
}

func (r *remoteIndex) Digest() (v1.Hash, error) {
	return partial.Digest(r)
}

func (r *remoteIndex) Size() (int64, error) {
	return partial.Size(r)
}

func (r *remoteIndex) RawManifest() ([]byte, error) {
	r.manifestLock.Lock()
	defer r.manifestLock.Unlock()
	if r.manifest != nil {
		return r.manifest, nil
	}

	// NOTE(jonjohnsonjr): We should never get here because the public entrypoints
	// do type-checking via remote.Descriptor. I've left this here for tests that
	// directly instantiate a remoteIndex.
	manifest, desc, err := r.fetchManifest(r.Ref, acceptableIndexMediaTypes)
	if err != nil {
		return nil, err
	}

	if r.descriptor == nil {
		r.descriptor = desc
	}
	r.mediaType = desc.MediaType
	r.manifest = manifest
	return r.manifest, nil
}

func (r *remoteIndex) IndexManifest() (*v1.IndexManifest, error) {
	return partial.IndexManifest(r)
}

func (r *remoteIndex) Image(h v1.Hash) (v1.Image, error) {
	desc, err := r.childByHash(h)
	if err != nil {
		return nil, err
	}

	// Descriptor.Image will handle coercing nested indexes into an Image.
	return desc.Image()
}

// Descriptor retains the original descriptor from an index manifest.
// See partial.Descriptor.
func (r *remoteIndex) Descriptor() (*v1.Descriptor, error) {
	// kind of a hack, but RawManifest does appropriate locking/memoization
	// and makes sure r.descriptor is populated.
	_, err := r.RawManifest()
	return r.descriptor, err
}

func (r *remoteIndex) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
	desc, err := r.childByHash(h)
	if err != nil {
		return nil, err
	}
	return desc.ImageIndex()
}

func (r *remoteIndex) childByHash(h v1.Hash) (*Descriptor, error) {
	index, err := r.IndexManifest()
	if err != nil {
		return nil, err
	}
	for _, childDesc := range index.Manifests {
		if h == childDesc.Digest {
			return r.childDescriptor(childDesc, defaultPlatform)
		}
	}
	return nil, fmt.Errorf("no child with digest %s in index %s", h, r.Ref)
}

// Convert one of this index's child's v1.Descriptor into a remote.Descriptor, with the given platform option.
func (r *remoteIndex) childDescriptor(child v1.Descriptor, platform v1.Platform) (*Descriptor, error) {
	ref := r.Ref.Context().Digest(child.Digest.String())
	manifest, _, err := r.fetchManifest(ref, []types.MediaType{child.MediaType})
	if err != nil {
		return nil, err
	}
	return &Descriptor{
		fetcher: fetcher{
			Ref:     ref,
			Client:  r.Client,
			context: r.context,
		},
		Manifest:   manifest,
		Descriptor: child,
		platform:   platform,
	}, nil
}
