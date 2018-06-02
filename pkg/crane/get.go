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
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func getImage(r string) (v1.Image, name.Reference, error) {
	ref, err := name.ParseReference(r, name.WeakValidation)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing reference %q: %v", r, err)
	}
	auth, err := authn.DefaultKeychain.Resolve(ref.Context().Registry)
	if err != nil {
		return nil, nil, fmt.Errorf("getting creds for %q: %v", ref, err)
	}
	img, err := remote.Image(ref, auth, http.DefaultTransport)
	if err != nil {
		return nil, nil, fmt.Errorf("reading image %q: %v", ref, err)
	}
	return img, ref, nil
}
