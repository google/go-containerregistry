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
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdPull()) }

// NewCmdPull creates a new cobra.Command for the pull subcommand.
func NewCmdPull() *cobra.Command {
	var cachePath, format string

	cmd := &cobra.Command{
		Use:   "pull IMAGE [IMAGE] [...] TARBALL",
		Short: "Pull one or more remote images by reference and store their contents in a tarball",
		Args:  cobra.MinimumNArgs(2),
		PreRun: func(cmd *cobra.Command, args []string) {
			if len(args) > 2 && format != "tarball" {
				log.Fatalf("saving multiple images is compatible only with tarball format")
			}
		},
		Run: func(_ *cobra.Command, args []string) {
			switch format {
			case "tarball":
				srcs, path := args[:len(args)-1], args[len(args)-1]
				srcToImage := make(map[string]v1.Image, len(srcs))

				for _, src := range srcs {
					img := pullImage(src, cachePath)
					srcToImage[src] = img
				}

				if err := crane.MultiSave(srcToImage, path); err != nil {
					log.Fatalf("saving tarball %s: %v", path, err)
				}
			case "legacy":
				src, path := args[0], args[1]
				img := pullImage(src, cachePath)

				if err := crane.SaveLegacy(img, src, path); err != nil {
					log.Fatalf("saving legacy tarball %s: %v", path, err)
				}
			case "oci":
				src, path := args[0], args[1]
				img := pullImage(src, cachePath)

				if err := crane.SaveOCI(img, path); err != nil {
					log.Fatalf("saving oci image layout %s: %v", path, err)
				}
			default:
				log.Fatalf("unexpected --format: %q (valid values are: tarball, legacy, and oci)", format)
			}
		},
	}
	cmd.Flags().StringVarP(&cachePath, "cache_path", "c", "", "Path to cache image layers")
	cmd.Flags().StringVar(&format, "format", "tarball", fmt.Sprintf("Format in which to save images (%q, %q, or %q)", "tarball", "legacy", "oci"))

	return cmd
}

// pullImage pulls an image from a registry.
func pullImage(src string, cachePath string) v1.Image {
	img, err := crane.Pull(src, options...)
	if err != nil {
		log.Fatal(err)
	}
	if cachePath != "" {
		img = cache.Image(img, cache.NewFilesystemCache(cachePath))
	}

	return img
}
