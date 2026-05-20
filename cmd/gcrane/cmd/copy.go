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
	"runtime"

	"github.com/google/go-containerregistry/pkg/gcrane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
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
		RunE: func(cc *cobra.Command, args []string) error {
			src, dst := args[0], args[1]
			ctx := cc.Context()
			opts := []gcrane.Option{
				gcrane.WithUserAgent(userAgent()),
				gcrane.WithContext(ctx),
			}
			platform, err := rootPlatform(cc)
			if err != nil {
				return err
			}
			if platform != nil {
				opts = append(opts, gcrane.WithPlatform(platform))
			}
			if recursive {
				opts = append(opts, gcrane.WithJobs(jobs))
				return gcrane.CopyRepository(ctx, src, dst, opts...)
			}
			return gcrane.Copy(src, dst, opts...)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Whether to recurse through repos")
	cmd.Flags().IntVarP(&jobs, "jobs", "j", runtime.GOMAXPROCS(0), "The maximum number of concurrent copies")

	return cmd
}

// rootPlatform reads the persistent --platform flag inherited from the crane
// root command. "all" and the empty string both mean "no platform filter".
func rootPlatform(cmd *cobra.Command) (*v1.Platform, error) {
	f := cmd.Root().PersistentFlags().Lookup("platform")
	if f == nil {
		return nil, nil
	}
	s := f.Value.String()
	if s == "" || s == "all" {
		return nil, nil
	}
	return v1.ParsePlatform(s)
}
