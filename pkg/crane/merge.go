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

package crane

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// MergeSource allows to specify a remote reference and any overrides that should be included in generated index
// Override is only applied if Source is an image (not an index), otherwise we cannot know which child image should get the override.
type MergeSource struct {
	Source   string
	Override *v1.Descriptor
}

// MakeMergeSources is a small helper to generate MergeSource when no modifications are required
func MakeMergeSources(sources []string) []MergeSource {
	ms := make([]MergeSource, 0, len(sources))
	for _, s := range sources {
		ms = append(ms, MergeSource{Source: s})
	}
	return ms
}

// Merge creates an index from list of paths (tarballs) and
func Merge(sources []MergeSource, opt ...Option) (*v1.ImageIndex, error) {
	o := makeOptions(opt...)

	idx, err := appendIndex(mutate.IndexMediaType(empty.Index, types.DockerManifestList), sources, o)
	if err != nil {
		return nil, fmt.Errorf("unable to create merged index, err: %v", err)
	}

	return &idx, nil
}

func appendIndex(base v1.ImageIndex, sources []MergeSource, o options) (v1.ImageIndex, error) {
	for _, src := range sources {
		desc, err := getRemoteDescriptor(src.Source, o)
		if err != nil {
			return nil, err
		}
		adds, err := indexAddendumFromRemote(desc, src.Override)
		if err != nil {
			return nil, err
		}

		base = mutate.AppendManifests(base, adds...)
	}

	return base, nil
}

func indexAddendumFromRemote(desc *remote.Descriptor, override *v1.Descriptor) ([]mutate.IndexAddendum, error) {
	adds := make([]mutate.IndexAddendum, 0)

	switch desc.MediaType {
	case types.OCIImageIndex, types.DockerManifestList:
		idx, err := desc.ImageIndex()
		if err != nil {
			return nil, err
		}

		im, err := idx.IndexManifest()
		if err != nil {
			return nil, err
		}

		for _, imDesc := range im.Manifests {
			// Currently only Image and ImageIndex are supported by mutate/index
			if imDesc.MediaType.IsImage() {
				i, err := idx.Image(imDesc.Digest)
				if err != nil {
					return nil, err
				}

				adds = append(adds, mutate.IndexAddendum{
					Add: i,
				})
			} else if imDesc.MediaType.IsIndex() {
				ii, err := idx.ImageIndex(imDesc.Digest)
				if err != nil {
					return nil, err
				}

				adds = append(adds, mutate.IndexAddendum{
					Add: ii,
				})
			} else {
				logs.Warn.Printf("Unsupported media type in Manifest: %v, won't be appended", imDesc.MediaType)
			}
		}
	case types.DockerManifestSchema1, types.DockerManifestSchema1Signed:
		return nil, fmt.Errorf("merging v1 manifest is not supported")
	default:
		img, err := desc.Image()
		if err != nil {
			return nil, err
		}
		cfg, err := img.ConfigFile()
		if err != nil {
			return nil, err
		}

		desc := v1.Descriptor{
			URLs:        desc.URLs,
			MediaType:   desc.MediaType,
			Annotations: desc.Annotations,
			Platform: &v1.Platform{
				Architecture: cfg.Architecture,
				OS:           cfg.OS,
				OSVersion:    cfg.OSVersion,
			},
		}
		if override != nil {
			desc = mergeImageDescriptors(desc, *override)
		}

		adds = append(adds, mutate.IndexAddendum{
			Add:        img,
			Descriptor: desc,
		})
	}

	return adds, nil
}

func getRemoteDescriptor(src string, o options) (*remote.Descriptor, error) {
	ref, err := name.ParseReference(src, o.name...)
	if err != nil {
		return nil, fmt.Errorf("parsing reference for %q: %v", src, err)
	}

	desc, err := remote.Get(ref, o.remote...)
	if err != nil {
		return nil, fmt.Errorf("fetching %q: %v", src, err)
	}

	return desc, nil
}

// We're only copying fields that make sense
func mergeImageDescriptors(base v1.Descriptor, override v1.Descriptor) v1.Descriptor {
	final := base
	if override.Annotations != nil {
		final.Annotations = override.Annotations
	}

	if override.Platform != nil {
		final.Platform = override.Platform
	}
	return final
}
