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

package crane

import (
	"io"
	"log"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdExport()) }

// NewCmdExport creates a new cobra.Command for the export subcommand.
func NewCmdExport() *cobra.Command {
	exportCmd := &cobra.Command{
		Use:   "export IMAGE OUTPUT",
		Short: "Export contents of a remote image as a tarball",
		Example: `  # Write tarball to stdout
  crane export ubuntu -

  # Write tarball to file
  crane export ubuntu ubuntu.tar`,
		Args: cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			doExport(args[0], args[1])
		},
	}

	return exportCmd
}

func openFile(s string) (*os.File, error) {
	if s == "-" {
		return os.Stdout, nil
	}
	return os.Create(s)
}

func doExport(src, dst string) {
	srcRef, err := name.ParseReference(src)
	if err != nil {
		log.Fatalf("parsing reference %q: %v", src, err)
	}
	img, err := remote.Image(srcRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		log.Fatalf("reading image %q: %v", srcRef, err)
	}

	fs := mutate.Extract(img)

	out, err := openFile(dst)
	if err != nil {
		log.Fatalf("failed to open %s: %v", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, fs); err != nil {
		log.Fatal(err)
	}
}
