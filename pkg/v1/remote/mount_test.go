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
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestMountableImage(t *testing.T) {
	img, err := random.Image(1024, 5)
	if err != nil {
		t.Fatal(err)
	}

	ref, err := name.ParseReference("ubuntu")
	if err != nil {
		t.Fatal(err)
	}

	img = &mountableImage{
		Image:     img,
		Reference: ref,
	}

	if err := validate.Image(img); err != nil {
		t.Errorf("Validate() = %v", err)
	}

	layers, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}

	for i, l := range layers {
		if _, ok := l.(*MountableLayer); !ok {
			t.Errorf("layers[%d] should be MountableLayer but isn't", i)
		}
	}
}
