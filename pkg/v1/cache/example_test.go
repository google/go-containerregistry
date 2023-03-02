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

package cache_test

import (
	"fmt"
	"log"
	"os"

	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

func ExampleImage() {
	img, err := random.Image(1024*1024, 3)
	if err != nil {
		log.Fatal(err)
	}
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		log.Fatal(err)
	}
	fs := cache.NewFilesystemCache(dir)

	// cached will cache layers from img using the fs cache
	cached := cache.Image(img, fs)

	// Use cached as you would use img.
	digest, err := cached.Digest()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(digest)
}
