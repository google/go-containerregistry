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

package remote

import (
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
)

// TODO(jonjohnson): This.
// // image accesses an image from a remote registry
// type image struct{}
// var _ v1.Image = (*image)(nil)

// Image accesses a given image reference over the provided transport, with the provided authentication.
func Image(ref name.Reference, auth authn.Authenticator, t http.RoundTripper) (v1.Image, error) {
	return nil, fmt.Errorf("NYI: remote.Image(%v)", ref)
}
