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

package gcrane

import (
	"net/http"

	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdGc()) }

func NewCmdGc() *cobra.Command {
	return &cobra.Command{
		Use:   "gc",
		Short: "List images that are not tagged",
		Args:  cobra.ExactArgs(1),
		Run:   gc,
	}
}

func gc(_ *cobra.Command, args []string) {
	r := args[0]
	repo, err := name.NewRepository(r, name.WeakValidation)
	if err != nil {
		log.Fatalln(err)
	}
	auth, err := authn.DefaultKeychain.Resolve(repo.Registry)
	if err != nil {
		log.Fatalln(err)
	}
	tags, err := google.List(repo, auth, http.DefaultTransport)
	if err != nil {
		log.Fatalln(err)
	}

	for digest, manifest := range tags.Manifests {
		if len(manifest.Tags) == 0 {
			fmt.Printf("%s@%s\n", repo, digest)
		}
	}
}
