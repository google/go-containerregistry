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
func Layers(a, b v1.Layer) error {
	digests := []v1.Hash{}
	diffids := []v1.Hash{}
	sizes := []int64{}
	mts := []types.MediaType{}
	errs := []string{}

	for _, layer := range []v1.Layer{a, b} {
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
	}

	if want, got := digests[0], digests[1]; want != got {
		errs = append(errs, fmt.Sprintf("a.Digest() != b.Digest(); %s != %s", want, got))
	}
	if want, got := diffids[0], diffids[1]; want != got {
		errs = append(errs, fmt.Sprintf("a.DiffID() != b.DiffID(); %s != %s", want, got))
	}
	if want, got := sizes[0], sizes[1]; want != got {
		errs = append(errs, fmt.Sprintf("a.Size() != b.Size(); %d != %d", want, got))
	}
	if want, got := mts[0], mts[1]; want != got {
		errs = append(errs, fmt.Sprintf("a.MediaType() != b.MediaType(); %s != %s", want, got))
	}

	if len(errs) != 0 {
		return errors.New("Layers differ:\n" + strings.Join(errs, "\n"))
	}

	return nil
}
