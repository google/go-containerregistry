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

package layout

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"golang.org/x/sync/errgroup"
)

var layoutFile = `{
    "imageLayoutVersion": "1.0.0"
}`

// AppendImage writes a v1.Image to an OCI image layout at path and updates
// the index.json to reference it.
func AppendImage(path string, img v1.Image, options ...LayoutOption) (v1.ImageIndex, error) {
	if err := WriteImage(path, img); err != nil {
		return nil, err
	}

	mt, err := img.MediaType()
	if err != nil {
		return nil, err
	}

	d, err := img.Digest()
	if err != nil {
		return nil, err
	}

	manifest, err := img.RawManifest()
	if err != nil {
		return nil, err
	}

	desc := v1.Descriptor{
		MediaType: mt,
		Size:      int64(len(manifest)),
		Digest:    d,
	}

	for _, opt := range options {
		if err := opt(&desc); err != nil {
			return nil, err
		}
	}

	return AppendDescriptor(path, desc)
}

// AppendIndex writes a v1.ImageIndex to an OCI image layout at path and updates
// the index.json to reference it.
func AppendIndex(path string, ii v1.ImageIndex, options ...LayoutOption) (v1.ImageIndex, error) {
	if err := WriteIndex(path, ii); err != nil {
		return nil, err
	}

	mt, err := ii.MediaType()
	if err != nil {
		return nil, err
	}

	d, err := ii.Digest()
	if err != nil {
		return nil, err
	}

	manifest, err := ii.RawIndexManifest()
	if err != nil {
		return nil, err
	}

	desc := v1.Descriptor{
		MediaType: mt,
		Size:      int64(len(manifest)),
		Digest:    d,
	}

	for _, opt := range options {
		if err := opt(&desc); err != nil {
			return nil, err
		}
	}

	return AppendDescriptor(path, desc)
}

// AppendDescriptor adds a descriptor to the index.json of an ImageIndex located
// at path.
func AppendDescriptor(path string, desc v1.Descriptor) (v1.ImageIndex, error) {
	// Create an empty image index if it doesn't exist.
	var ii v1.ImageIndex
	ii, err := Index(path)
	if os.IsNotExist(err) {
		if err := writeFile(path, "oci-layout", []byte(layoutFile)); err != nil {
			return nil, err
		}
		ii = empty.Index
	}

	index, err := ii.IndexManifest()
	if err != nil {
		return nil, err
	}

	index.Manifests = append(index.Manifests, desc)

	rawIndex, err := json.MarshalIndent(index, "", "   ")
	if err != nil {
		return nil, err
	}

	if err := writeFile(path, "index.json", rawIndex); err != nil {
		return nil, err
	}

	return &layoutIndex{
		path:     path,
		rawIndex: rawIndex,
	}, nil
}

func writeFile(path string, name string, data []byte) error {
	if err := os.MkdirAll(path, os.ModePerm); err != nil && !os.IsExist(err) {
		return err
	}

	return ioutil.WriteFile(filepath.Join(path, name), data, os.ModePerm)
}

// WriteBlob copies a file to the blobs/ directory from the given ReadCloser at
// blobs/{hash.Algorithm}/{hash.Hex}.
func WriteBlob(path string, hash v1.Hash, r io.ReadCloser) error {
	dir := filepath.Join(path, "blobs", hash.Algorithm)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil && !os.IsExist(err) {
		return err
	}

	w, err := os.Create(filepath.Join(dir, hash.Hex))
	if os.IsExist(err) {
		// Blob already exists, that's fine.
		return nil
	} else if err != nil {
		return err
	}
	defer w.Close()

	_, err = io.Copy(w, r)
	return err
}

// TODO: A streaming version of WriteBlob so we don't have to know the hash
// before we write it.

// TODO: For streaming layers we should write to a tmp file then Rename to the
// final digest.
func writeLayer(path string, layer v1.Layer) error {
	d, err := layer.Digest()
	if err != nil {
		return err
	}

	r, err := layer.Compressed()
	if err != nil {
		return err
	}

	return WriteBlob(path, d, r)
}

// WriteImage writes a v1.Image to an OCI Image layout at path.
// This doesn't update the top-level index.json, see AppendImage.
func WriteImage(path string, img v1.Image) error {
	layers, err := img.Layers()
	if err != nil {
		return err
	}

	// Write the layers concurrently.
	var g errgroup.Group
	for _, layer := range layers {
		layer := layer
		g.Go(func() error {
			return writeLayer(path, layer)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Write the config.
	cfgName, err := img.ConfigName()
	if err != nil {
		return err
	}
	cfgBlob, err := img.RawConfigFile()
	if err != nil {
		return err
	}
	if err := WriteBlob(path, cfgName, ioutil.NopCloser(bytes.NewReader(cfgBlob))); err != nil {
		return err
	}

	// Write the img manifest.
	d, err := img.Digest()
	if err != nil {
		return err
	}
	manifest, err := img.RawManifest()
	if err != nil {
		return err
	}

	return WriteBlob(path, d, ioutil.NopCloser(bytes.NewReader(manifest)))
}

func writeIndexToFile(path string, indexFile string, ii v1.ImageIndex) error {
	index, err := ii.IndexManifest()
	if err != nil {
		return err
	}

	// Walk the descriptors and write any v1.Image or v1.ImageIndex that we find.
	// If we come across something we don't expect, just write it as a blob.
	for _, desc := range index.Manifests {
		switch desc.MediaType {
		case types.OCIImageIndex, types.DockerManifestList:
			ii, err := ii.ImageIndex(desc.Digest)
			if err != nil {
				return err
			}
			if err := WriteIndex(path, ii); err != nil {
				return err
			}
		case types.OCIManifestSchema1, types.DockerManifestSchema2:
			img, err := ii.Image(desc.Digest)
			if err != nil {
				return err
			}
			if err := WriteImage(path, img); err != nil {
				return err
			}
		default:
			// We don't recognize this artifact, just pass it through.
			blob, err := ii.Blob(desc.Digest)
			if err != nil {
				return err
			}
			if err := WriteBlob(path, desc.Digest, blob); err != nil {
				return err
			}
		}
	}

	rawIndex, err := ii.RawIndexManifest()
	if err != nil {
		return err
	}

	return writeFile(path, indexFile, rawIndex)
}

// WriteIndex writes a v1.ImageIndex to an OCI Image layout at path.
// This doesn't update the top-level index.json, see AppendIndex.
func WriteIndex(path string, ii v1.ImageIndex) error {
	// Always just write oci-layout file, since it's small.
	if err := writeFile(path, "oci-layout", []byte(layoutFile)); err != nil {
		return err
	}

	h, err := ii.Digest()
	if err != nil {
		return err
	}

	indexFile := filepath.Join("blobs", h.Algorithm, h.Hex)
	return writeIndexToFile(path, indexFile, ii)
}

// Write converts an ImageIndex to an OCI image layout at path.
//
// The contents are written in the following format:
// At the top level, there is:
//   One oci-layout file containing the version of this image-layout.
//   One index.json file listing decsriptors for the contained images.
// Under blobs/, there is, for each image:
//   One file for each layer, named after the layer's SHA.
//   One file for each config blob, named after its SHA.
//   One file for each manifest blob, named after its SHA.
func Write(path string, ii v1.ImageIndex) error {
	// Always just write oci-layout file, since it's small.
	if err := writeFile(path, "oci-layout", []byte(layoutFile)); err != nil {
		return err
	}

	return writeIndexToFile(path, "index.json", ii)
}
