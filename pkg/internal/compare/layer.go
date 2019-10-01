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
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Layers compares the given layers to each other and returns an error if they
// differ.  Note that this does not compare the actual contents (by calling
// Compressed or Uncompressed).
func Layers(layers ...v1.Layer) error {
	if len(layers) < 2 {
		return fmt.Errorf("comparing %d layers makes no sense", len(layers))
	}

	digests := []v1.Hash{}
	diffids := []v1.Hash{}
	sizes := []int64{}
	mts := []types.MediaType{}
	errs := []string{}

	for i, layer := range layers {
		digest, err := layer.Digest()
		if err != nil {
			return err
		}
		digests = append(digests, digest)

		diffid, err := layer.DiffID()
		if err != nil {
			return err
		}
		diffids = append(diffids, diffid)

		size, err := layer.Size()
		if err != nil {
			return err
		}
		sizes = append(sizes, size)

		mt, err := layer.MediaType()
		if err != nil {
			return err
		}
		mts = append(mts, mt)

		if i > 0 {
			if want, got := digests[i-1], digests[i]; want != got {
				errs = append(errs, fmt.Sprintf("layer[%d].Digest() != layer[%d].Digest(); %s != %s", i-1, i, want, got))
			}
			if want, got := diffids[i-1], diffids[i]; want != got {
				errs = append(errs, fmt.Sprintf("layer[%d].DiffID() != layer[%d].DiffID(); %s != %s", i-1, i, want, got))
			}
			if want, got := sizes[i-1], sizes[i]; want != got {
				errs = append(errs, fmt.Sprintf("layer[%d].Size() != layer[%d].Size(); %d != %d", i-1, i, want, got))
			}
			if want, got := mts[i-1], mts[i]; want != got {
				errs = append(errs, fmt.Sprintf("layer[%d].MediaType() != layer[%d].MediaType(); %s != %s", i-1, i, want, got))
			}
		}
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n\n"))
	}

	return nil
}
