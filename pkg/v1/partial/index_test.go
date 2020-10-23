// Copyright 2020 Google LLC All Rights Reserved.
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

package partial_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

func mustImage(t *testing.T) v1.Image {
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestImageByPlatform(t *testing.T) {
	img1, img2, img3 := mustImage(t), mustImage(t), mustImage(t)

	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{
			Add: img1,
			// No Platform.
		},
		mutate.IndexAddendum{
			Add:        img2,
			Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "cat", Architecture: "kitten"}},
		},
		mutate.IndexAddendum{
			Add:        img3,
			Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "dog", Architecture: "puppy", Variant: "corgi"}},
		})

	// Images without platform assume amd64/linux.
	if got, err := partial.ImageByPlatform(idx, v1.Platform{OS: "linux", Architecture: "amd64"}); err != nil {
		t.Fatal(err)
	} else if got != img1 {
		t.Fatalf("ImageByPlatform(amd64,linux) got %v, want %v", got, img1)
	}

	if got, err := partial.ImageByPlatform(idx, v1.Platform{OS: "cat", Architecture: "kitten"}); err != nil {
		t.Fatal(err)
	} else if got != img2 {
		t.Fatalf("ImageByPlatform(cat,kitten) got %v, want %v", got, img2)
	}

	if got, err := partial.ImageByPlatform(idx, v1.Platform{OS: "dog", Architecture: "puppy", Variant: "corgi"}); err != nil {
		t.Fatal(err)
	} else if got != img3 {
		t.Fatalf("ImageByPlatform(dog,puppy,corgi) got %v, want %v", got, img3)
	}
}
