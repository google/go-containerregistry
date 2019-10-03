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
	"context"
	"io"
	"io/ioutil"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
)

// ImageLoader is an interface for testing.
type ImageLoader interface {
	ImageLoad(context.Context, io.Reader, bool) (types.ImageLoadResponse, error)
	ImageTag(context.Context, string, string) error
}

// GetImageLoader is a variable so we can override in tests.
var GetImageLoader = func() (ImageLoader, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	cli.NegotiateAPIVersion(context.Background())
	return cli, nil
}

// Tag adds a tag to an already existent image.
func Tag(src, dest name.Tag) error {
	cli, err := GetImageLoader()
	if err != nil {
		return err
	}

	return cli.ImageTag(context.Background(), src.String(), dest.String())
}

// Write saves the image into the daemon as the given tag.
func Write(tag name.Tag, img v1.Image) (string, error) {
	filter, err := probeIncremental(tag, img)
	if err != nil {
		logs.Warn.Printf("Determining incremental load: %v", err)
		return write(tag, img, keepLayers)
	}
	return write(tag, img, filter)
}

func write(tag name.Tag, img v1.Image, lf tarball.LayerFilter) (string, error) {
	cli, err := GetImageLoader()
	if err != nil {
		return "", err
	}

	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(tarball.Write(tag, img, pw, tarball.WithLayerFilter(lf)))
	}()

	// write the image in docker save format first, then load it
	resp, err := cli.ImageLoad(context.Background(), pr, false)
	if err != nil {
		return "", errors.Wrapf(err, "error loading image")
	}
	defer resp.Body.Close()
	b, readErr := ioutil.ReadAll(resp.Body)
	response := string(b)
	if readErr != nil {
		return response, errors.Wrapf(err, "error reading load response body")
	}
	return response, nil
}

func discardLayers(v1.Layer) (bool, error) {
	return false, nil
}

func keepLayers(v1.Layer) (bool, error) {
	return true, nil
}

func probeIncremental(tag name.Tag, img v1.Image) (tarball.LayerFilter, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}

	// Set<DiffID>
	have := make(map[v1.Hash]struct{})

	for i := 1; i < len(layers); i++ {
		// Image with first i layers.
		probe, err := mutate.AppendLayers(empty.Image, layers[0:i]...)
		if err != nil {
			return nil, err
		}

		if _, err := write(tag, probe, discardLayers); err != nil {
			return func(layer v1.Layer) (bool, error) {
				diffid, err := layers[i].DiffID()
				if err != nil {
					return true, err
				}

				if _, ok := have[diffid]; ok {
					return false, nil
				}

				return true, nil
			}, nil
		}

		// We don't need to include this layer in the tarball.
		diffid, err := layers[i].DiffID()
		if err != nil {
			return nil, err
		}
		have[diffid] = struct{}{}
	}

	return discardLayers, nil
}
