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

package daemon

import (
	"fmt"
	"io"

	"github.com/google/go-containerregistry/v1/types"

	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
)

// image accesses an image from a docker daemon
type image struct {
	cli *client.Client
}

var _ v1.Image = (*image)(nil)

// Image exposes an image reference from within the Docker daemon.
func Image(ref name.Reference) (v1.Image, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	img := &image{
		cli: cli,
	}
	return img, fmt.Errorf("NYI: daemon.Image(%v)", ref)
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
	panic("not implemented")
}

func (i *image) Manifest() (*v1.Manifest, error) {
	panic("not implemented")
}

func (i *image) RawManifest() ([]byte, error) {
	panic("not implemented")
}

func (i *image) ConfigFile() (*v1.ConfigFile, error) {
	panic("not implemented")
}

func (i *image) RawConfigFile() ([]byte, error) {
	panic("not implemented")
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
