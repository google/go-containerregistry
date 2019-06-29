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

package api

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"log"
)

func Delete(refStr string) {
	ref, err := name.ParseReference(refStr)
	if err != nil {
		log.Fatalf("parsing reference %q: %v", refStr, err)
	}

	if err := remote.Delete(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
		log.Fatalf("deleting image %q: %v", ref, err)
	}
}
