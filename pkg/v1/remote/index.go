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
	"bytes"
	"fmt"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// remoteIndex accesses an index from a remote registry
type remoteIndex struct {
	fetcher
	manifestLock sync.Mutex // Protects manifest
	manifest     []byte
	mediaType    types.MediaType
}

// Index provides access to a remote index reference, applying functional options
// to the underlying imageOpener before resolving the reference into a v1.ImageIndex.
func Index(ref name.Reference, options ...ImageOption) (v1.ImageIndex, error) {
	acceptable := []types.MediaType{
		types.DockerManifestList,
		types.OCIImageIndex,
	}

	desc, err := get(ref, acceptable, options...)
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

func (r *remoteIndex) RawManifest() ([]byte, error) {
	r.manifestLock.Lock()
	defer r.manifestLock.Unlock()
	if r.manifest != nil {
		return r.manifest, nil
	}

	acceptable := []types.MediaType{
		types.DockerManifestList,
		types.OCIImageIndex,
	}
	manifest, desc, err := r.fetchManifest(r.Ref, acceptable)
	if err != nil {
		return nil, err
	}

	r.mediaType = desc.MediaType
	r.manifest = manifest
	return r.manifest, nil
}

func (r *remoteIndex) IndexManifest() (*v1.IndexManifest, error) {
	b, err := r.RawManifest()
	if err != nil {
		return nil, err
	}
	return v1.ParseIndexManifest(bytes.NewReader(b))
}

func (r *remoteIndex) Image(h v1.Hash) (v1.Image, error) {
	imgRef, err := name.ParseReference(fmt.Sprintf("%s@%s", r.Ref.Context(), h), name.StrictValidation)
	if err != nil {
		return nil, err
	}
	ri := &remoteImage{
		fetcher: fetcher{
			Ref:    imgRef,
			Client: r.Client,
		},
	}
	imgCore, err := partial.CompressedToImage(ri)
	if err != nil {
		return imgCore, err
	}
	// Wrap the v1.Layers returned by this v1.Image in a hint for downstream
	// remote.Write calls to facilitate cross-repo "mounting".
	return &mountableImage{
		Image:     imgCore,
		Reference: r.Ref,
	}, nil
}

func (r *remoteIndex) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
	idxRef, err := name.ParseReference(fmt.Sprintf("%s@%s", r.Ref.Context(), h), name.StrictValidation)
	if err != nil {
		return nil, err
	}
	return &remoteIndex{
		fetcher: fetcher{
			Ref:    idxRef,
			Client: r.Client,
		},
	}, nil
}

// This naively matches the first manifest with matching Architecture and OS.
//
// We should probably use this instead:
//	 github.com/containerd/containerd/platforms
//
// But first we'd need to migrate to:
//   github.com/opencontainers/image-spec/specs-go/v1
func (r *remoteIndex) ImageByPlatform(platform v1.Platform) (v1.Image, error) {
	desc, err := r.DescriptorByPlatform(platform)
	if err != nil {
		return nil, err
	}

	// Descriptor.Image will call back into here if it's an index.
	return desc.Image()
}

func (r *remoteIndex) DescriptorByPlatform(platform v1.Platform) (*Descriptor, error) {
	index, err := r.IndexManifest()
	if err != nil {
		return nil, err
	}
	for _, childDesc := range index.Manifests {
		// If platform is missing from child descriptor, assume it's amd64/linux.
		p := defaultPlatform
		if childDesc.Platform != nil {
			p = *childDesc.Platform
		}
		if platform.Architecture == p.Architecture && platform.OS == p.OS {
			childRef, err := r.childRef(childDesc.Digest)
			if err != nil {
				return nil, err
			}
			manifest, desc, err := r.fetchManifest(childRef, []types.MediaType{childDesc.MediaType})
			if err != nil {
				return nil, err
			}

			return &Descriptor{
				fetcher: fetcher{
					Ref:    childRef,
					Client: r.Client,
				},
				Manifest:   manifest,
				Descriptor: *desc,
				platform:   platform,
			}, nil
		}
	}
	return nil, fmt.Errorf("no matching image for %s/%s in %s", platform.Architecture, platform.OS, r.Ref)
}

func (r *remoteIndex) childRef(h v1.Hash) (name.Reference, error) {
	return name.ParseReference(fmt.Sprintf("%s@%s", r.Ref.Context(), h), name.StrictValidation)
}
