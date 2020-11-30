// Copyright 2019 Google LLC All Rights Reserved.
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
	"log"
	"runtime"

	"github.com/google/go-containerregistry/pkg/gcrane"
	"github.com/spf13/cobra"
)

// NewCmdCopy creates a new cobra.Command for the copy subcommand.
func NewCmdCopy() *cobra.Command {
	recursive := false
	jobs := 1
	cmd := &cobra.Command{
		Use:     "copy SRC DST",
		Aliases: []string{"cp"},
		Short:   "Efficiently copy a remote image from src to dst",
		Args:    cobra.ExactArgs(2),
		Run: func(cc *cobra.Command, args []string) {
			src, dst := args[0], args[1]
			if recursive {
				// We should wire this up to signal handlers and make sure we
				// respect the cancellation downstream.
				ctx := context.TODO()
				if err := gcrane.CopyRepository(ctx, src, dst, gcrane.WithJobs(jobs)); err != nil {
					log.Fatal(err)
				}
			} else {
				if err := gcrane.Copy(src, dst); err != nil {
					log.Fatal(err)
				}
			}
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Whether to recurse through repos")
	cmd.Flags().IntVarP(&jobs, "jobs", "j", runtime.GOMAXPROCS(0), "The maximum number of concurrent copies")

	return cmd
}
