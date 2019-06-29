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

func init() { Root.AddCommand(NewCmdPush()) }

// NewCmdPush creates a new cobra.Command for the push subcommand.
func NewCmdPush() *cobra.Command {
	return &cobra.Command{
		Use:   "push",
		Short: "Push image contents as a tarball to a remote registry",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			api.Push(args[0], args[1])
		},
	}
}
