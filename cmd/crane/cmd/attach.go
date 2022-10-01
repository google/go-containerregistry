// Copyright 2022 Google LLC All Rights Reserved.
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
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"
)

// NewCmdAttach creates a new cobra.Command for the attach subcommand.
func NewCmdAttach(options *[]crane.Option) *cobra.Command {
	var attachType, fn string

	attachmentsCmd := &cobra.Command{
		Use:   "attach",
		Short: "Add an attachment to an image",
		// TODO: Long
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			refstr := args[0]
			b, err := os.ReadFile(fn)
			if err != nil {
				return err
			}
			return crane.Attach(refstr, b, attachType, *options...)
		},
	}
	attachmentsCmd.Flags().StringVarP(&attachType, "type", "t", "", "Type of attachment")
	attachmentsCmd.Flags().StringVarP(&fn, "file", "f", "", "Name of file to attach")
	attachmentsCmd.MarkFlagRequired("type")
	attachmentsCmd.MarkFlagRequired("file")
	return attachmentsCmd
}
