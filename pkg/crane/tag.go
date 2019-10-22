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

package crane

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Tag adds tag to the remote img.
func Tag(img, tag string) error {
	ref, err := name.ParseReference(img)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", img, err)
	}
	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return fmt.Errorf("fetching %q: %v", img, err)
	}

	dst := ref.Context().Tag(tag)

	return remote.Tag(dst, &wrap{desc}, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

// wrap coerces a remote.Descriptor into a remote.Taggable.
type wrap struct {
	desc *remote.Descriptor
}

func (w *wrap) RawManifest() ([]byte, error)        { return w.desc.Manifest, nil }
func (w *wrap) MediaType() (types.MediaType, error) { return w.desc.MediaType, nil }
func (w *wrap) Digest() (v1.Hash, error)            { return w.desc.Digest, nil }
