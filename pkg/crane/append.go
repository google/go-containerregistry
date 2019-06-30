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

package crane

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"log"
)

func Append(src string, dst string, tar string, output string) {
	srcRef, err := name.ParseReference(src)
	if err != nil {
		log.Fatalf("parsing reference %q: %v", src, err)
	}
	srcImage, err := remote.Image(srcRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		log.Fatalf("reading image %q: %v", srcRef, err)
	}

	dstTag, err := name.NewTag(dst)
	if err != nil {
		log.Fatalf("parsing tag %q: %v", dst, err)
	}

	layer, err := tarball.LayerFromFile(tar)
	if err != nil {
		log.Fatalf("reading tar %q: %v", tar, err)
	}

	image, err := mutate.AppendLayers(srcImage, layer)
	if err != nil {
		log.Fatalf("appending layer: %v", err)
	}

	if output != "" {
		if err := tarball.WriteToFile(output, dstTag, image); err != nil {
			log.Fatalf("writing output %q: %v", output, err)
		}
		return
	}

	if err := remote.Write(dstTag, image, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
		log.Fatalf("writing image %q: %v", dstTag, err)
	}
}
