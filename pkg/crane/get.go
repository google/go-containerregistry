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

package crane

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func getImage(refStr string) (v1.Image, name.Reference, error) {
	reference, err := name.ParseReference(refStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing reference %q: %v", refStr, err)
	}
	img, err := remote.Image(reference, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, nil, fmt.Errorf("reading image %q: %v", reference, err)
	}
	return img, reference, nil
}

func getManifest(refStr string) (*remote.Descriptor, error) {
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return nil, fmt.Errorf("parsing reference %q: %v", refStr, err)
	}
	return remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}
