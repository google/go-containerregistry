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

package main

import (
	"fmt"
	"os"

	crane "github.com/google/go-containerregistry/cmd/crane/cmd"
	gcrane "github.com/google/go-containerregistry/cmd/gcrane/cmd"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/spf13/cobra"
)

func init() {
	logs.Warn.SetOutput(os.Stderr)
	logs.Progress.SetOutput(os.Stderr)
}

func main() {
	crane.Root.Use = "gcrane"
	crane.Root.Short = "gcrane is a tool for managing container images on gcr.io and pkg.dev"

	gcraneCmds := []*cobra.Command{gcrane.NewCmdList(), gcrane.NewCmdGc(), gcrane.NewCmdCopy()}

	// Maintain a map of google-specific commands that we "override".
	used := make(map[string]bool)
	for _, cmd := range gcraneCmds {
		used[cmd.Use] = true
	}

	// Use crane for everything else so that this can be a drop-in replacement.
	for _, cmd := range crane.Root.Commands() {
		if _, ok := used[cmd.Use]; ok {
			crane.Root.RemoveCommand(cmd)
		}
	}

	for _, cmd := range gcraneCmds {
		crane.Root.AddCommand(cmd)
	}

	if err := crane.Root.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
