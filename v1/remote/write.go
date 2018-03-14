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

// WriteOptions are used to expose optional information to guide or
// control the image write.
type WriteOptions struct {
	// The set of paths from which to attempt to mount blobs.
	MountPaths []name.Repository
	// TODO(mattmoor): Expose "threads" to limit parallelism?
}

// Write pushes the provided img to the specified image reference.
func Write(ref name.Reference, img v1.Image, auth authn.Authenticator, t http.RoundTripper,
	wo WriteOptions) error {
	return fmt.Errorf("NYI: remote.Write(%v)", ref)
}
