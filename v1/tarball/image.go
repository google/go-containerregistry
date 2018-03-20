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
	"os"
	"sync"

	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/partial"
	"github.com/google/go-containerregistry/v1/types"
	"github.com/google/go-containerregistry/v1/v1util"
)

type image struct {
	path          string
	td            *tarDescriptor
	config        *v1.ConfigFile
	imgDescriptor *singleImageTarDescriptor

	manifestLock sync.Mutex // Protects manifest
	manifest     *v1.Manifest

	tag *name.Tag
}

var _ partial.UncompressedImageCore = (*image)(nil)

// Image exposes an image from the tarball at the provided path.
func Image(path string, tag *name.Tag) (v1.Image, error) {
	img := image{
		path: path,
		tag:  tag,
	}
	if err := img.loadTarDescriptorAndConfig(); err != nil {
		return nil, err
	}
	return partial.UncompressedToImage(&img)
}

func (i *image) ConfigName() (v1.Hash, error) {
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(i.config); err != nil {
		return v1.Hash{}, err
	}
	h, _, err := v1.SHA256(v1util.NopReadCloser(buf))
	return h, err
}

func (i *image) MediaType() (types.MediaType, error) {
	return types.DockerManifestSchema2, nil
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
	td, err := extractFileFromTar(i.path, "manifest.json")
	if err != nil {
		return err
	}
	defer td.Close()

	if err := json.NewDecoder(td).Decode(&i.td); err != nil {
		return err
	}

	i.imgDescriptor, err = i.td.findSpecifiedImageDescriptor(i.tag)
	if err != nil {
		return err
	}

	cfg, err := extractFileFromTar(i.path, i.imgDescriptor.Config)
	if err != nil {
		return err
	}

	i.config, err = v1.ParseConfigFile(cfg)
	if err != nil {
		return err
	}
	return nil
}

// tarFile represents a single file inside a tar. Closing it closes the tar itself.
type tarFile struct {
	io.Reader
	io.Closer
}

func extractFileFromTar(tarPath string, filePath string) (io.ReadCloser, error) {
	f, err := os.Open(tarPath)
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
			return tarFile{
				Reader: tf,
				Closer: f,
			}, nil
		}
	}
	return nil, fmt.Errorf("file %s not found in tar", filePath)
}

func (i *image) ConfigFile() (*v1.ConfigFile, error) {
	return i.config, nil
}

func (i *image) UncompressedLayer(h v1.Hash) (io.ReadCloser, error) {
	for idx, diffID := range i.config.RootFS.DiffIDs {
		if diffID == h {
			return extractFileFromTar(i.path, i.imgDescriptor.Layers[idx])
		}
	}
	return nil, fmt.Errorf("diff id %q not found", h)
}
