// Copyright 2022 Google LLC All Rights Reserved.
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

package mutate

import (
	"fmt"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Drops docker specific properties
// See: https://github.com/opencontainers/image-spec/blob/main/config.md
func toOCIV1Config(config v1.Config) v1.Config {
	return v1.Config{
		User:         config.User,
		ExposedPorts: config.ExposedPorts,
		Env:          config.Env,
		Entrypoint:   config.Entrypoint,
		Cmd:          config.Cmd,
		Volumes:      config.Volumes,
		WorkingDir:   config.WorkingDir,
		Labels:       config.Labels,
		StopSignal:   config.StopSignal,
	}
}

func toOCIV1ConfigFile(cf *v1.ConfigFile) *v1.ConfigFile {
	return &v1.ConfigFile{
		Created:      cf.Created,
		Author:       cf.Author,
		Architecture: cf.Architecture,
		OS:           cf.OS,
		OSVersion:    cf.OSVersion,
		History:      cf.History,
		RootFS:       cf.RootFS,
		Config:       toOCIV1Config(cf.Config),
	}
}

// OCIImage mutates the provided v1.Image to be OCI compliant v1.Image
// Check image-spec to see which properties are ported and which are dropped.
// https://github.com/opencontainers/image-spec/blob/main/config.md
func OCIImage(base v1.Image) (v1.Image, error) {
	// Get original manifest
	m, err := base.Manifest()
	if err != nil {
		return nil, err
	}
	// Convert config
	cfg, err := base.ConfigFile()
	if err != nil {
		return nil, err
	}
	cfg = toOCIV1ConfigFile(cfg)

	layers, err := base.Layers()
	if err != nil {
		return nil, err
	}

	newLayers := []v1.Layer{}

	for _, layer := range layers {
		mediaType, err := layer.MediaType()
		if err != nil {
			return nil, err
		}
		switch mediaType {
		case types.DockerLayer:
			reader, err := layer.Compressed()
			if err != nil {
				return nil, fmt.Errorf("getting layer: %w", err)
			}
			layer, err = tarball.LayerFromOpener(func() (io.ReadCloser, error) { return reader, nil }, tarball.WithMediaType(types.OCILayer))
			if err != nil {
				return nil, fmt.Errorf("building layer: %w", err)
			}
		case types.DockerUncompressedLayer:
			reader, err := layer.Uncompressed()
			if err != nil {
				return nil, fmt.Errorf("getting layer: %w", err)
			}
			layer, err = tarball.LayerFromOpener(func() (io.ReadCloser, error) { return reader, nil }, tarball.WithMediaType(types.OCIUncompressedLayer))
			if err != nil {
				return nil, fmt.Errorf("building layer: %w", err)
			}
		}
		newLayers = append(newLayers, layer)
	}

	base, err = AppendLayers(empty.Image, newLayers...)
	if err != nil {
		return nil, err
	}

	base = MediaType(base, types.OCIManifestSchema1)
	base = ConfigMediaType(base, types.OCIConfigJSON)
	base = Annotations(base, m.Annotations).(v1.Image)
	base, err = ConfigFile(base, cfg)
	if err != nil {
		return nil, err
	}
	return base, nil
}

// OCIImageIndex mutates the provided v1.ImageIndex to be OCI compliant v1.ImageIndex
func OCIImageIndex(base v1.ImageIndex) (v1.ImageIndex, error) {
	base = IndexMediaType(base, types.OCIImageIndex)
	mn, err := base.IndexManifest()
	if err != nil {
		return nil, err
	}

	removals := []v1.Hash{}
	addendums := []IndexAddendum{}

	for _, manifest := range mn.Manifests {
		if !manifest.MediaType.IsImage() {
			// it is not an image, leave it as is
			continue
		}
		img, err := base.Image(manifest.Digest)
		if err != nil {
			return nil, err
		}
		img, err = OCIImage(img)
		if err != nil {
			return nil, err
		}
		mt, err := img.MediaType()
		if err != nil {
			return nil, err
		}
		removals = append(removals, manifest.Digest)
		addendums = append(addendums, IndexAddendum{Add: img, Descriptor: v1.Descriptor{
			URLs:        manifest.URLs,
			MediaType:   mt,
			Annotations: manifest.Annotations,
			Platform:    manifest.Platform,
		}})
	}
	base = RemoveManifests(base, match.Digests(removals...))
	base = AppendManifests(base, addendums...)
	return base, nil
}
