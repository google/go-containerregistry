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
	"strings"

	"github.com/Masterminds/semver"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
)

// NewCmdList creates a new cobra.Command for the ls subcommand.
func NewCmdList(options *[]crane.Option) *cobra.Command {
	var fullRef, omitDigestTags bool
	var versionConstraint string
	cmd := &cobra.Command{
		Use:   "ls REPO",
		Short: "List the tags in a repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			repo := args[0]
			tags, err := crane.ListTags(repo, *options...)
			if err != nil {
				return fmt.Errorf("reading tags for %s: %w", repo, err)
			}

			r, err := name.NewRepository(repo)
			if err != nil {
				return err
			}

			if len(versionConstraint) > 0 {
				constraint, err := semver.NewConstraint(versionConstraint)
				if err != nil {
					return fmt.Errorf("invalid version constraint %v: %w", versionConstraint, err)
				}

				for i := len(tags) - 1; i >= 0; i-- {
					tag := tags[i]
					version, err := semver.NewVersion(tag)
					if err != nil {
						return fmt.Errorf("tag is not a valid semver %v: %w", tag, err)
					}
					valid, _ := constraint.Validate(version)
					if !valid {
						tags = append(tags[:i], tags[i+1:]...)
					}
				}

			}

			for _, tag := range tags {
				if omitDigestTags && strings.HasPrefix(tag, "sha256-") {
					continue
				}

				if fullRef {
					fmt.Println(r.Tag(tag))
				} else {
					fmt.Println(tag)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&fullRef, "full-ref", false, "(Optional) if true, print the full image reference")
	cmd.Flags().BoolVar(&omitDigestTags, "omit-digest-tags", false, "(Optional), if true, omit digest tags (e.g., ':sha256-...')")
	cmd.Flags().StringVar(&versionConstraint, "version-constraint", "", "(Optional), when specified, list only tags that satisfy the version constraint (e.g., '>1.0.0')")
	return cmd
}
