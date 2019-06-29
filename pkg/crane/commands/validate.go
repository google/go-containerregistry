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

package commands

import (
	"github.com/google/go-containerregistry/pkg/crane/api"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdValidate()) }

// NewCmdValidate creates a new cobra.Command for the validate subcommand.
func NewCmdValidate() *cobra.Command {
	var tarballPath, remoteRef, daemonRef string
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate that an image is well-formed",
		Args:  cobra.ExactArgs(0),
		Run: func(_ *cobra.Command, args []string) {
			api.Validate(tarballPath, remoteRef, daemonRef)
		},
	}
	validateCmd.Flags().StringVar(&tarballPath, "tarball", "", "Path to tarball to validate")
	validateCmd.Flags().StringVar(&remoteRef, "remote", "", "Name of remote image to validate")
	validateCmd.Flags().StringVar(&daemonRef, "daemon", "", "Name of image in daemon to validate")

	return validateCmd
}
