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
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"log"
)

func Push(src string, dst string) {
	t, err := name.NewTag(dst)
	if err != nil {
		log.Fatalf("parsing tag %q: %v", dst, err)
	}
	log.Printf("Pushing %v", t)

	i, err := tarball.ImageFromPath(src, nil)
	if err != nil {
		log.Fatalf("reading image %q: %v", src, err)
	}

	if err := remote.Write(t, i, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
		log.Fatalf("writing image %q: %v", t, err)
	}
}
