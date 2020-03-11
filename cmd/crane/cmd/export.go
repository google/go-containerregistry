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
	"log"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdExport()) }

// NewCmdExport creates a new cobra.Command for the export subcommand.
func NewCmdExport() *cobra.Command {
	var oci bool

	cmd := &cobra.Command{
		Use:   "export IMAGE DEST",
		Short: "Export contents of a remote image as a tarball or oci-image layout",
		Example: `  # Write tarball to stdout
  crane export ubuntu -

  # Write tarball to file
  crane export ubuntu ubuntu.tar

  # Write image as oci layout
  crane export ubuntu --oci ./ubuntu`,
		Args: cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			src, dst := args[0], args[1]

			img, err := crane.Pull(src, options...)
			if err != nil {
				log.Fatal(err)
			}

			if !oci {
				f, err := openFile(dst)
				if err != nil {
					log.Fatalf("failed to open %s: %v", dst, err)
				}
				defer f.Close()
				if err := crane.Export(img, f); err != nil {
					log.Fatalf("exporting %s: %v", src, err)
				}
			} else {
				crane.SaveOCI(img, dst)
			}
		}}

	f := cmd.Flags()
	f.BoolVarP(&oci, "oci", "", false, "export image in oci layout format")

	return cmd
}

func openFile(s string) (*os.File, error) {
	if s == "-" {
		return os.Stdout, nil
	}
	return os.Create(s)
}
