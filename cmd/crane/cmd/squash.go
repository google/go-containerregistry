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

package cmd

import (
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdSquash()) }

// NewCmdSquash creates a new cobra.Command for the squash subcommand.
func NewCmdSquash() *cobra.Command {
	var orig, squashed string

	squashCmd := &cobra.Command{
		Use:   "squash",
		Short: "Squash an image onto a new base image",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			origImg, err := crane.Pull(orig, options...)
			if err != nil {
				log.Fatalf("pulling %s: %v", orig, err)
			}

			img, err := mutate.Squash(origImg)
			if err != nil {
				log.Fatalf("squashing: %v", err)
			}

			if err := crane.Push(img, squashed, options...); err != nil {
				log.Fatalf("pushing %s: %v", squashed, err)
			}

			digest, err := img.Digest()
			if err != nil {
				log.Fatalf("digesting squashed: %v", err)
			}
			fmt.Println(digest.String())
		},
	}
	squashCmd.Flags().StringVarP(&orig, "original", "", "", "Original image to squash")
	squashCmd.Flags().StringVarP(&squashed, "squashed", "", "", "Tag to apply to squashed image")

	squashCmd.MarkFlagRequired("original")
	squashCmd.MarkFlagRequired("squashed")
	return squashCmd
}

