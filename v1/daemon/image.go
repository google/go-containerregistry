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
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/google/go-containerregistry/v1/tarball"

	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
)

// image accesses an image from a docker daemon
type image struct {
	cli *client.Client
	v1.Image
}

var _ v1.Image = (*image)(nil)

// Image exposes an image reference from within the Docker daemon.
func Image(ref name.Reference) (v1.Image, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	rc, err := cli.ImageSave(context.Background(), []string{ref.String()})
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	imageBytes, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	opener := func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(imageBytes)), nil
	}

	tb, err := tarball.Image(opener, nil)
	if err != nil {
		return nil, err
	}
	img := &image{
		cli:   cli,
		Image: tb,
	}
	return img, fmt.Errorf("NYI: daemon.Image(%v)", ref)
}
