// Copyright 2021 Google LLC All Rights Reserved.
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

package cmd

import (
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdFlatten()) }

// NewCmdFlatten creates a new cobra.Command for the flatten subcommand.
func NewCmdFlatten() *cobra.Command {
	var orig, flattened string

	flattenCmd := &cobra.Command{
		Use:   "flatten",
		Short: "Flatten an image into a new image with a single layer",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			origImg, err := crane.Pull(orig, options...)
			if err != nil {
				log.Fatalf("pulling %s: %v", orig, err)
			}

			img, err := mutate.Flatten(origImg)
			if err != nil {
				log.Fatalf("flattening: %v", err)
			}

			if err := crane.Push(img, flattened, options...); err != nil {
				log.Fatalf("pushing %s: %v", flattened, err)
			}

			digest, err := img.Digest()
			if err != nil {
				log.Fatalf("digesting flattened: %v", err)
			}
			fmt.Println(digest.String())
		},
	}
	flattenCmd.Flags().StringVarP(&orig, "original", "", "", "Original image to flatten")
	flattenCmd.Flags().StringVarP(&flattened, "flattened", "", "", "Tag to apply to flattened image")

	flattenCmd.MarkFlagRequired("original")
	flattenCmd.MarkFlagRequired("flattened")
	return flattenCmd
}