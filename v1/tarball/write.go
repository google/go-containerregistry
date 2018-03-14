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

package tarball

import (
	"fmt"

	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
)

// WriteOptions are used to expose optional information to guide or
// control the image write.
type WriteOptions struct {
	// TODO(mattmoor): Whether to store things compressed?
}

// Write saves the image as the given tag in a tarball at the given path.
func Write(p string, tag name.Tag, img v1.Image, wo WriteOptions) error {
	return fmt.Errorf("NYI: tarball.Write(%v)", p)
}
