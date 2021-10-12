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

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"
)

// NewCmdPush creates a new cobra.Command for the push subcommand.
func NewCmdPush(options *[]crane.Option) *cobra.Command {
	var concurrent int
	cmd := &cobra.Command{
		Use:   "push TARBALL IMAGE",
		Short: "Push a tarball contains one or more image(s) to a remote registry",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			path, target := args[0], args[1]
			// target contain "/" is a tag or reference, load it as single image
			if strings.Contains(target, "/") {
				image, err := crane.Load(path, *options...)
				if err != nil {
					return fmt.Errorf("loading %s as tarball: %#v", path, err)
				}
				return crane.Push(image, target, *options...)
			}
			images, err := crane.LoadMulti(path, *options...)
			if err != nil {
				return fmt.Errorf("loading %s as tarball: %#v", path, err)
			}
			return crane.MultiPush(images, target, concurrent, *options...)
		},
	}
	cmd.Flags().IntVarP(&concurrent, "concurrent", "c", 10, "Set the number of threads pushing image at the same time")
	return cmd
}
