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

package tarball

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"

	"github.com/google/go-containerregistry/compress"
	"github.com/google/go-containerregistry/name"

	"github.com/google/go-containerregistry/v1/types"

	"github.com/google/go-containerregistry/v1"
)

type image struct {
	path           string
	descriptorLock sync.Mutex // Protects td, config and imgDescriptor
	td             *tarDescriptor
	config         *v1.ConfigFile
	imgDescriptor  *singleImageTarDescriptor

	manifestLock sync.Mutex // Protects manifest
	manifest     *v1.Manifest

	tag *name.Tag
}

var _ v1.Image = (*image)(nil)

// Image exposes an image from the tarball at the provided path.
func Image(path string, tag *name.Tag) (v1.Image, error) {
	img := image{
		path: path,
		tag:  tag,
	}
	return &img, nil
}

func (i *image) maybeLoadManifest() error {
	if i.manifest != nil {
		return nil
	}
	return i.loadManifestAndBlobs()
}

func (i *image) FSLayers() ([]v1.Hash, error) {
	panic("not implemented")
}

func (i *image) DiffIDs() ([]v1.Hash, error) {
	panic("not implemented")
}

func (i *image) ConfigName() (v1.Hash, error) {
	panic("not implemented")
}

func (i *image) BlobSet() (map[v1.Hash]struct{}, error) {
	panic("not implemented")
}

func (i *image) Digest() (v1.Hash, error) {
	panic("not implemented")
}

func (i *image) MediaType() (types.MediaType, error) {
	if err := i.maybeLoadManifest(); err != nil {
		return types.MediaType(""), err
	}
	return i.manifest.MediaType, nil
}

// Manifest returns the v1.Manifest for the specified image.
// This method memoizes the result, avoiding repeated reads from the tarball.
func (i *image) Manifest() (*v1.Manifest, error) {
	if err := i.maybeLoadManifest(); err != nil {
		return nil, err
	}
	return i.manifest, nil
}

// singleImageTarDescriptor is the struct used to represent a single image inside a `docker save` tarball.
type singleImageTarDescriptor struct {
	Config   string
	RepoTags []string
	Layers   []string
}

// tarDescriptor is the struct used inside the `manifest.json` file of a `docker save` tarball.
type tarDescriptor []singleImageTarDescriptor

func (td tarDescriptor) findSpecifiedImageDescriptor(tag *name.Tag) (*singleImageTarDescriptor, error) {
	if tag == nil {
		if len(td) != 1 {
			return nil, errors.New("tarball must contain only a single image to be used with tarball.Image")
		}
		return &(td)[0], nil
	}
	for _, img := range td {
		for _, tagStr := range img.RepoTags {
			repoTag, err := name.NewTag(tagStr, name.WeakValidation)
			if err != nil {
				return nil, err
			}

			// Compare the resolved names, since there are several ways to specify the same tag.
			if repoTag.Name() == tag.Name() {
				return &img, nil
			}
		}
	}
	return nil, fmt.Errorf("tag %s not found in tarball", tag)
}

func (i *image) loadTarDescriptorAndConfig() error {
	i.descriptorLock.Lock()
	defer i.descriptorLock.Unlock()
	tdBytes, err := extractFileFromTar(i.path, "manifest.json")
	if err != nil {
		return err
	}

	if err := json.Unmarshal(tdBytes, &i.td); err != nil {
		return err
	}

	i.imgDescriptor, err = i.td.findSpecifiedImageDescriptor(i.tag)
	if err != nil {
		return err
	}

	cfgBytes, err := extractFileFromTar(i.path, i.imgDescriptor.Config)
	if err != nil {
		return err
	}

	i.config, err = v1.ParseConfigFile(cfgBytes)
	if err != nil {
		return err
	}
	return nil
}

func (i *image) loadManifestAndBlobs() error {
	i.manifestLock.Lock()
	defer i.manifestLock.Unlock()

	// Generating the manifest requires the config file to be parsed.
	if i.config == nil {
		if err := i.loadTarDescriptorAndConfig(); err != nil {
			return err
		}
	}

	cfgBytes, err := json.Marshal(i.config)
	if err != nil {
		return err
	}
	manifest := v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.DockerManifestSchema2,
		Config: v1.Descriptor{
			MediaType: types.DockerConfigJSON,
			Size:      int64(len(cfgBytes)),
			Digest:    v1.SHA256(string(cfgBytes)),
		},
	}

	for _, l := range i.imgDescriptor.Layers {
		// TODO(dlorenc): support compressed layers.
		uncompressed, err := extractFileFromTar(i.path, l)
		if err != nil {
			return err
		}
		r, err := compress.Compress(bytes.NewReader(uncompressed))
		if err != nil {
			return err
		}
		layer, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}

		manifest.Layers = append(manifest.Layers, v1.Descriptor{
			MediaType: types.DockerLayer,
			Size:      int64(len(layer)),
			Digest:    v1.SHA256(string(layer)),
		})
	}
	i.manifest = &manifest
	return nil
}

func extractFileFromTar(tarPath string, filePath string) ([]byte, error) {
	f, err := os.Open(tarPath)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	tf := tar.NewReader(f)
	for {
		hdr, err := tf.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == filePath {
			var buf bytes.Buffer
			_, err := io.Copy(&buf, tf)
			if err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("file %s not found in tar", filePath)
}

func (i *image) ConfigFile() (*v1.ConfigFile, error) {
	if i.config == nil {
		if err := i.loadTarDescriptorAndConfig(); err != nil {
			return nil, err
		}
	}
	return i.config, nil
}

func (i *image) BlobSize(v1.Hash) (int64, error) {
	panic("not implemented")
}

func (i *image) Blob(v1.Hash) (io.ReadCloser, error) {
	panic("not implemented")
}

func (i *image) Layer(v1.Hash) (io.ReadCloser, error) {
	panic("not implemented")
}

func (i *image) UncompressedBlob(v1.Hash) (io.ReadCloser, error) {
	panic("not implemented")
}

func (i *image) UncompressedLayer(v1.Hash) (io.ReadCloser, error) {
	panic("not implemented")
}
