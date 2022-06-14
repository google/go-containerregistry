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
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/gcrane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/spf13/cobra"
)

// NewCmdGct creates a new cobra.Command for the gct subcommand.
func NewCmdGct() *cobra.Command {
	recursive := false
	cmd := &cobra.Command{
		Use:   "gct",
		Short: "List images that are tagged",
		Args:  cobra.ExactArgs(1),
		RunE: func(cc *cobra.Command, args []string) error {
			return gct(cc.Context(), args[0], recursive)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Whether to recurse through repos")

	return cmd
}

func gct(ctx context.Context, root string, recursive bool) error {
	repo, err := name.NewRepository(root)
	if err != nil {
		return err
	}

	opts := []google.Option{
		google.WithAuthFromKeychain(gcrane.Keychain),
		google.WithUserAgent(userAgent()),
		google.WithContext(ctx),
	}

	if recursive {
		return google.Walk(repo, printTaggedImages, opts...)
	}

	tags, err := google.List(repo, opts...)
	return printTaggedImages(repo, tags, err)
}

func printTaggedImages(repo name.Repository, tags *google.Tags, err error) error {
	if err != nil {
		return err
	}

	for digest, manifest := range tags.Manifests {
		if len(manifest.Tags) != 0 {
			for _, tag := range manifest.Tags {
			    fmt.Printf("%s@%s,%s:%s\n", repo, digest, repo, tag)
			}
		}
	}

	return nil
}
