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

package build

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

var (
	testImage, _ = random.Image(1024, 5)
)

func TestFixed(t *testing.T) {
	f := NewFixed(map[string]v1.Image{
		"asdf": testImage,
	})

	if got, want := f.IsSupportedReference("asdf"), true; got != want {
		t.Errorf("IsSupportedReference(asdf) = %v, want %v", got, want)
	}
	if got, err := f.Build("asdf"); err != nil {
		t.Errorf("Build(asdf) = %v, want %v", err, testImage)
	} else if got != testImage {
		t.Errorf("Build(asdf) = %v, want %v", got, testImage)
	}

	if got, want := f.IsSupportedReference("blah"), false; got != want {
		t.Errorf("IsSupportedReference(blah) = %v, want %v", got, want)
	}
	if got, err := f.Build("blah"); err == nil {
		t.Errorf("Build(blah) = %v, want error", got)
	}
}
