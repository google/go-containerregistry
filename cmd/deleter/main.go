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

package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1/remote"
)

var (
	ref = flag.String("ref", "", "The reference to delete from its registry.")
)

func parseReference(ref string) (name.Reference, error) {
	tag, err := name.NewTag(ref, name.WeakValidation)
	if err == nil {
		return tag, nil
	}
	return name.NewDigest(ref, name.WeakValidation)
}

func main() {
	flag.Parse()
	if *ref == "" {
		log.Fatalln("-ref must be specified.")
	}

	r, err := parseReference(*ref)
	if err != nil {
		log.Fatalln(err)
	}

	auth, err := authn.DefaultKeychain.Resolve(r.Context().Registry)
	if err != nil {
		log.Fatalln(err)
	}

	if err := remote.Delete(r, auth, http.DefaultTransport, remote.DeleteOptions{}); err != nil {
		log.Fatalln(err)
	}
}
