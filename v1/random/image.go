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

package random

import (
	"fmt"

	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
)

// TODO(mattmoor): This.
// // image is pseudo-randomly generated.
// type image struct{}
// var _ v1.Image = (*image)(nil)

// Image returns a pseudo-randomly generated Image.
func Image(byteSize, layers uint) (v1.Image, error) {
	return nil, fmt.Errorf("NYI: random.Image(%v)", ref)
}
