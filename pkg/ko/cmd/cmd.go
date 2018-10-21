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
	"github.com/spf13/cobra"
	"io"
	"os"
)

func NewDefaultKoCommand() *cobra.Command {
	return NewDefaultKoCommandWithArgs(&defaultExtensionHandler{}, os.Environ(), os.Args, os.Stdin, os.Stdout, os.Stderr)
}

func NewDefaultKoCommandWithArgs(extensionHandler ExtensionHandler, env, args []string, in io.Reader, out, errout io.Writer) *cobra.Command {
	cmd := NewKoCommand(env, args, in, out, errout)

	if extensionHandler == nil {
		return cmd
	}

	if len(args) > 1 {
		cmdPathPieces := args[1:]

		// only look for suitable extension executables if
		// the specified command does not already exist
		if _, _, err := cmd.Find(cmdPathPieces); err != nil {
			if err := handleExtensions(extensionHandler, env, cmdPathPieces); err != nil {
				fmt.Fprintf(errout, "%v\n", err)
				os.Exit(1)
			}
		}
	}

	return cmd
}

func NewKoCommand(env, args []string, in io.Reader, out, err io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   "ko",
		Short: "Rapidly iterate with Go, Containers, and Kubernetes.",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	addKubeCommands(cmds, env, args, in, out, err)
	return cmds
}
