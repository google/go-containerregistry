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
	"github.com/google/go-containerregistry/v1/tarball"
)

func init() {
	var pullSrc, pullDst string
	pullCmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull a remote image by reference and store its contents in a tarball",
		Run: func(cmd *cobra.Command, args []string) {
			t, err := name.NewTag(pullSrc, name.WeakValidation)
			if err != nil {
				log.Fatalln(err)
			}
			log.Printf("Pulling %v", t)

			auth, err := authn.DefaultKeychain.Resolve(t.Registry)
			if err != nil {
				log.Fatalln(err)
			}

			i, err := remote.Image(t, auth, http.DefaultTransport)
			if err != nil {
				log.Fatalln(err)
			}

			if err := tarball.Write(pullDst, t, i, &tarball.WriteOptions{}); err != nil {
				log.Fatalln(err)
			}
		},
	}
	pullCmd.Flags().StringVarP(&pullSrc, "src", "s", "", "Remote image reference to pull from")
	pullCmd.Flags().StringVarP(&pullDst, "dst", "d", "", "Path to tarball to write")
	rootCmd.AddCommand(pullCmd)
}
