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

	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestDifferentLayers(t *testing.T) {
	a, err := random.Layer(100, types.DockerLayer)
	if err != nil {
		t.Fatal(err)
	}
	b, err := random.Layer(100, types.OCILayer)
	if err != nil {
		t.Fatal(err)
	}

	if err := Layers(a, b); err == nil {
		t.Errorf("got nil err, should have something")
	}
}

func TestEqualLayers(t *testing.T) {
	a, err := random.Layer(100, types.DockerLayer)
	if err != nil {
		t.Fatal(err)
	}

	if err := Layers(a, a); err != nil {
		t.Errorf("got err: %v", err)
	}
}
