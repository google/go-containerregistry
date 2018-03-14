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

package daemon

import (
	"fmt"

	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
)

// TODO(dlorenc): This.
// // image accesses an image from a remote registry
// type image struct{}
// var _ v1.Image = (*image)(nil)

// Image exposes an image reference from within the Docker daemon.
func Image(ref name.Reference) (v1.Image, error) {
	return nil, fmt.Errorf("NYI: daemon.Image(%v)", ref)
}
