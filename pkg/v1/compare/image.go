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

// Images compares the given images to each other and returns an error if they
// differ.
func Images(a, b v1.Image) error {
	digests := []v1.Hash{}
	manifests := []*v1.Manifest{}
	cns := []v1.Hash{}
	sizes := []int64{}
	mts := []types.MediaType{}
	layerss := [][]v1.Layer{}

	errs := []string{}

	for _, img := range []v1.Image{a, b} {
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
	}

	if want, got := digests[0], digests[1]; want != got {
		errs = append(errs, fmt.Sprintf("a.Digest() != b.Digest(); %s != %s", want, got))
	}
	if want, got := cns[0], cns[1]; want != got {
		errs = append(errs, fmt.Sprintf("a.ConfigName() != b.ConfigName(); %s != %s", want, got))
	}
	if want, got := manifests[0], manifests[1]; !reflect.DeepEqual(want, got) {
		errs = append(errs, fmt.Sprintf("a.Manifest() != b.Manifest(); %v != %v", want, got))
	}
	if want, got := sizes[0], sizes[1]; want != got {
		errs = append(errs, fmt.Sprintf("a.Size() != b.Size(); %d != %d", want, got))
	}
	if want, got := mts[0], mts[1]; want != got {
		errs = append(errs, fmt.Sprintf("a.MediaType() != b.MediaType(); %s != %s", want, got))
	}

	if len(layerss[0]) != len(layerss[1]) {
		// If we have fewer layers than the first image, abort with an error so we don't panic.
		return errors.New("len(a.Layers()) != len(b.Layers())")
	}

	// Compare each layer.
	for i := 0; i < len(layerss[0]); i++ {
		if err := Layers(layerss[0][i], layerss[1][i]); err != nil {
			// Wrap the error in newlines to delineate layer errors.
			errs = append(errs, fmt.Sprintf("Layers[%d]: %v\n", i, err))
		}
	}

	if len(errs) != 0 {
		return errors.New("Images differ:\n" + strings.Join(errs, "\n"))
	}

	return nil
}
