// Copyright 2024 Google LLC All Rights Reserved.
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
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/cobra"
)

// NewCmdBlob creates a new cobra.Command for the blob subcommand.
func NewCmdBlobCalculate(options *[]crane.Option) *cobra.Command {
	return &cobra.Command{
		Use:     "calculate BLOB",
		Short:   "Calculate the diffid and digest for a blob",
		Example: "crane blob calculate ./layer.tar.gz",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := args[0]
			layer, err := getLayer(p)
			if err != nil {
				return fmt.Errorf("reading layer %s: %w", p, err)
			}

			var diffid, digest v1.Hash

			differr := make(chan error)
			digesterr := make(chan error)
			go func() {
				diffid, err = layer.DiffID()
				differr <- err
			}()

			go func() {
				digest, err = layer.Digest()
				digesterr <- err
			}()

			if err := <-differr; err != nil {
				return fmt.Errorf("getting diffid %s: %w", p, err)
			}

			if err := <-digesterr; err != nil {
				return fmt.Errorf("getting digest %s: %w", p, err)
			}

			fmt.Printf("%s %s\n", diffid, digest)

			return nil
		},
	}
}

func getLayer(path string) (v1.Layer, error) {
	f, err := streamFile(path)
	if err != nil {
		return nil, err
	}
	if f != nil {
		return stream.NewLayer(f), nil
	}

	return tarball.LayerFromFile(path)
}

// If we're dealing with a named pipe, trying to open it multiple times will
// fail, so we need to do a streaming upload.
//
// returns nil, nil for non-streaming files
func streamFile(path string) (*os.File, error) {
	if path == "-" {
		return os.Stdin, nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, nil
	}

	if !fi.Mode().IsRegular() {
		return os.Open(path)
	}

	return nil, nil
}
