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

func init() { Root.AddCommand(NewCmdList()) }

func NewCmdList() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List the contents of a repo",
		Args:  cobra.ExactArgs(1),
		Run:   ls,
	}
}

func ls(_ *cobra.Command, args []string) {
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

	// Track what we saw in the response so we can fall back to non-GCR behavior.
	gcrSpecific := false

	for _, child := range tags.Children {
		fmt.Printf("%s/%s\n", repo, child)
	}

	for digest, manifest := range tags.Manifests {
		gcrSpecific = true
		fmt.Printf("%s@%s\n", repo, digest)

		// For GCR, print the tags immediately after the digests they point to.
		for _, tag := range manifest.Tags {
			fmt.Printf("%s:%s\n", repo, tag)
		}
	}

	if !gcrSpecific {
		// If we didn't see any GCR extensions, just list the tags like normal.
		for _, tag := range tags.Tags {
			fmt.Printf("%s:%s\n", repo, tag)
		}
	}
}
