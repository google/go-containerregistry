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

// Indexes compares the given indexes to each other and returns an error if
// they differ.
func Indexes(a, b v1.ImageIndex) error {
	digests := []v1.Hash{}
	manifests := []*v1.IndexManifest{}
	sizes := []int64{}
	mts := []types.MediaType{}

	errs := []string{}

	for _, idx := range []v1.ImageIndex{a, b} {
		digest, err := idx.Digest()
		if err != nil {
			return err
		}
		digests = append(digests, digest)

		manifest, err := idx.IndexManifest()
		if err != nil {
			return err
		}
		manifests = append(manifests, manifest)

		size, err := idx.Size()
		if err != nil {
			return err
		}
		sizes = append(sizes, size)

		mt, err := idx.MediaType()
		if err != nil {
			return err
		}
		mts = append(mts, mt)
	}

	if want, got := digests[0], digests[1]; want != got {
		errs = append(errs, fmt.Sprintf("a.Digest() != b.Digest(); %s != %s", want, got))
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

	// TODO(jonjohnsonjr): Iterate over Manifest and compare Image and ImageIndex results.

	if len(errs) != 0 {
		return errors.New("Indexes differ:\n" + strings.Join(errs, "\n"))
	}

	return nil
}
