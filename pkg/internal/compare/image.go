// Copyright 2019 Google LLC All Rights Reserved.
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

package compare

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func Images(imgs ...v1.Image) error {
	if len(imgs) < 2 {
		return fmt.Errorf("comparing %d images makes no sense", len(imgs))
	}

	digests := []v1.Hash{}
	manifests := []*v1.Manifest{}
	cns := []v1.Hash{}
	sizes := []int64{}
	mts := []types.MediaType{}
	layerss := [][]v1.Layer{}

	errs := []string{}

	for i, img := range imgs {
		layers, err := img.Layers()
		if err != nil {
			return err
		}
		layerss = append(layerss, layers)

		digest, err := img.Digest()
		if err != nil {
			return err
		}
		digests = append(digests, digest)

		manifest, err := img.Manifest()
		if err != nil {
			return err
		}
		manifests = append(manifests, manifest)

		cn, err := img.ConfigName()
		if err != nil {
			return err
		}
		cns = append(cns, cn)

		size, err := img.Size()
		if err != nil {
			return err
		}
		sizes = append(sizes, size)

		mt, err := img.MediaType()
		if err != nil {
			return err
		}
		mts = append(mts, mt)

		if i > 0 {
			if want, got := digests[i-1], digests[i]; want != got {
				errs = append(errs, fmt.Sprintf("image[%d].Digest() != image[%d].Digest(); %s != %s", i-1, i, want, got))
			}
			if want, got := cns[i-1], cns[i]; want != got {
				errs = append(errs, fmt.Sprintf("image[%d].ConfigName() != image[%d].ConfigName(); %s != %s", i-1, i, want, got))
			}
			if want, got := manifests[i-1], manifests[i]; !reflect.DeepEqual(want, got) {
				errs = append(errs, fmt.Sprintf("image[%d].Manifest() != image[%d].Manifest(); %v != %v", i-1, i, want, got))
			}
			if want, got := sizes[i-1], sizes[i]; want != got {
				errs = append(errs, fmt.Sprintf("image[%d].Size() != image[%d].Size(); %d != %d", i-1, i, want, got))
			}
			if want, got := mts[i-1], mts[i]; want != got {
				errs = append(errs, fmt.Sprintf("image[%d].MediaType() != image[%d].MediaType(); %s != %s", i-1, i, want, got))
			}
		}
	}

	// Compare layers all at once.
	for i := 0; i < len(layerss[0]); i++ {
		layers := []v1.Layer{}
		for j := 0; j < len(layerss); j++ {
			if len(layerss[j]) > i {
				layers = append(layers, layerss[j][i])
			} else {
				// If we have fewer layers than the first image, report an error,
				// even though something else will catch it, we don't have to panic.
				errs = append(errs, fmt.Sprintf("len(image[%d].Layers()) < len(image[0].Layers())", j))
			}
		}
		if err := Layers(layers...); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n\n"))
	}

	return nil
}
