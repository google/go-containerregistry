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

package mutate

import (
	"encoding/json"
	"errors"

	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/partial"
	"github.com/google/go-containerregistry/v1/types"
)

// Addendum contains layers and history to be appended
// to a base image
type Addendum struct {
	Layer   v1.Layer
	History v1.History
}

// AppendLayers applies layers to a base image
func AppendLayers(base v1.Image, layers ...v1.Layer) (v1.Image, error) {
	additions := make([]Addendum, 0, len(layers))
	for _, layer := range layers {
		additions = append(additions, Addendum{Layer: layer})
	}

	return Append(base, additions...)
}

// Append will apply the list of addendums to the base image
func Append(base v1.Image, adds ...Addendum) (v1.Image, error) {
	if len(adds) == 0 {
		return base, nil
	}

	if err := validate(adds); err != nil {
		return nil, err
	}

	m, err := base.Manifest()
	if err != nil {
		return nil, err
	}

	cf, err := base.ConfigFile()
	if err != nil {
		return nil, err
	}

	image := &image{
		Image:      base,
		manifest:   m.DeepCopy(),
		configFile: cf.DeepCopy(),
		diffIDMap:  make(map[v1.Hash]v1.Layer),
		digestMap:  make(map[v1.Hash]v1.Layer),
	}

	diffIDs := image.configFile.RootFS.DiffIDs
	history := image.configFile.History

	for _, add := range adds {
		diffID, err := add.Layer.DiffID()
		if err != nil {
			return nil, err
		}
		diffIDs = append(diffIDs, diffID)
		history = append(history, add.History)
		image.diffIDMap[diffID] = add.Layer
	}

	manifestLayers := image.manifest.Layers

	for _, add := range adds {
		d := v1.Descriptor{
			MediaType: types.DockerLayer,
		}

		if d.Size, err = add.Layer.Size(); err != nil {
			return nil, err
		}

		if d.Digest, err = add.Layer.Digest(); err != nil {
			return nil, err
		}

		manifestLayers = append(manifestLayers, d)
		image.digestMap[d.Digest] = add.Layer
	}

	image.configFile.RootFS.DiffIDs = diffIDs
	image.configFile.History = history
	image.manifest.Layers = manifestLayers
	image.manifest.Config.Digest, err = image.ConfigName()
	if err != nil {
		return nil, err
	}

	return image, nil
}

// Config mutates the provided v1.Image to have the provided v1.Config
func Config(base v1.Image, cfg v1.Config) (v1.Image, error) {
	m, err := base.Manifest()
	if err != nil {
		return nil, err
	}

	cf, err := base.ConfigFile()
	if err != nil {
		return nil, err
	}

	cf.Config = cfg

	image := &image{
		Image:      base,
		manifest:   m.DeepCopy(),
		configFile: cf.DeepCopy(),
		diffIDMap:  make(map[v1.Hash]v1.Layer),
		digestMap:  make(map[v1.Hash]v1.Layer),
	}
	image.manifest.Config.Digest, err = image.ConfigName()
	if err != nil {
		return nil, err
	}
	return image, nil
}

type image struct {
	v1.Image
	configFile *v1.ConfigFile
	manifest   *v1.Manifest
	diffIDMap  map[v1.Hash]v1.Layer
	digestMap  map[v1.Hash]v1.Layer
}

// Layers returns the ordered collection of filesystem layers that comprise this image.
// The order of the list is oldest/base layer first, and most-recent/top layer last.
func (i *image) Layers() ([]v1.Layer, error) {
	diffIDs, err := partial.DiffIDs(i)
	if err != nil {
		return nil, err
	}
	ls := make([]v1.Layer, 0, len(diffIDs))
	for _, h := range diffIDs {
		l, err := i.LayerByDiffID(h)
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)
	}
	return ls, nil
}

// BlobSet returns an unordered collection of all the blobs in the image.
func (i *image) BlobSet() (map[v1.Hash]struct{}, error) {
	return partial.BlobSet(i)
}

// ConfigName returns the hash of the image's config file.
func (i *image) ConfigName() (v1.Hash, error) {
	return partial.ConfigName(i)
}

// ConfigFile returns this image's config file.
func (i *image) ConfigFile() (*v1.ConfigFile, error) {
	return i.configFile, nil
}

// RawConfigFile returns the serialized bytes of ConfigFile()
func (i *image) RawConfigFile() ([]byte, error) {
	return json.Marshal(i.configFile)
}

// Digest returns the sha256 of this image's manifest.
func (i *image) Digest() (v1.Hash, error) {
	return partial.Digest(i)
}

// Manifest returns this image's Manifest object.
func (i *image) Manifest() (*v1.Manifest, error) {
	return i.manifest, nil
}

// RawManifest returns the serialized bytes of Manifest()
func (i *image) RawManifest() ([]byte, error) {
	return json.Marshal(i.manifest)
}

// LayerByDigest returns a Layer for interacting with a particular layer of
// the image, looking it up by "digest" (the compressed hash).
func (i *image) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	if cn, err := i.ConfigName(); err != nil {
		return nil, err
	} else if h == cn {
		return partial.ConfigLayer(i)
	}
	if layer, ok := i.digestMap[h]; ok {
		return layer, nil
	}
	return i.Image.LayerByDigest(h)
}

// LayerByDiffID is an analog to LayerByDigest, looking up by "diff id"
// (the uncompressed hash).
func (i *image) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	if layer, ok := i.diffIDMap[h]; ok {
		return layer, nil
	}
	return i.Image.LayerByDiffID(h)
}

func validate(adds []Addendum) error {
	for _, add := range adds {
		if add.Layer == nil {
			return errors.New("Unable to add a nil layer to the image")
		}
	}
	return nil
}
