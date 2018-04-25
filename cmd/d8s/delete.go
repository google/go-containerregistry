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
	"log"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1/remote"
)

func init() {
	var ref string
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an image reference from its registry",
		Run: func(*cobra.Command, []string) {
			delette(ref)
		},
	}
	deleteCmd.Flags().StringVarP(&ref, "ref", "", "", "Image reference to delete")
}

func delette(ref string) {
	if ref == "" {
		log.Fatalln("Must provide --ref")
	}

	r, err := name.ParseReference(ref, name.WeakValidation)
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
