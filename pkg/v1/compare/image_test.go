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
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestDifferentImages(t *testing.T) {
	a, err := random.Image(100, 3)
	if err != nil {
		t.Fatal(err)
	}
	b, err := random.Image(100, 3)
	if err != nil {
		t.Fatal(err)
	}

	b = mutate.MediaType(b, types.OCIManifestSchema1)

	if err := Images(a, b); err == nil {
		t.Errorf("got nil err, should have something")
	}
}

func TestMismatchedLayers(t *testing.T) {
	a, err := random.Image(100, 3)
	if err != nil {
		t.Fatal(err)
	}
	b, err := random.Image(100, 2)
	if err != nil {
		t.Fatal(err)
	}

	if err := Images(a, b); err == nil {
		t.Errorf("got nil err, should have something")
	}
}

func TestEqualImages(t *testing.T) {
	a, err := random.Image(100, 2)
	if err != nil {
		t.Fatal(err)
	}

	if err := Images(a, a); err != nil {
		t.Errorf("got err: %v", err)
	}
}
