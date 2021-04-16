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
	"github.com/spf13/cobra"
)

// NewCmdDigest creates a new cobra.Command for the digest subcommand.
func NewCmdDigest(options *[]crane.Option) *cobra.Command {
	var tarball string
	cmd := &cobra.Command{
		Use:   "digest IMAGE",
		Short: "Get the digest of an image",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if tarball == "" && len(args) == 0 {
				cmd.Help()
				log.Fatalf("image reference required without --tarball")
			}

			digest, err := getDigest(tarball, args, options)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(digest)
		},
	}

	cmd.Flags().StringVar(&tarball, "tarball", "", "(Optional) path to tarball containing the image")

	return cmd
}

func getDigest(tarball string, args []string, options *[]crane.Option) (string, error) {
	if tarball != "" {
		return getTarballDigest(tarball, args, options)
	}

	return crane.Digest(args[0], *options...)
}

func getTarballDigest(tarball string, args []string, options *[]crane.Option) (string, error) {
	tag := ""
	if len(args) > 0 {
		tag = args[0]
	}

	img, err := crane.LoadTag(tarball, tag, *options...)
	if err != nil {
		return "", fmt.Errorf("loading image from %q: %v", tarball, err)
	}
	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("computing digest: %v", err)
	}
	return digest.String(), nil
}
