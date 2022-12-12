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
	"context"
	"os"
	"os/signal"

	"github.com/google/go-containerregistry/cmd/crane/cmd"
	gcmd "github.com/google/go-containerregistry/cmd/gcrane/cmd"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/gcrane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/spf13/cobra"
)

func init() {
	logs.Warn.SetOutput(os.Stderr)
	logs.Progress.SetOutput(os.Stderr)
}

const (
	use   = "gcrane"
	short = "gcrane is a tool for managing container images on gcr.io and pkg.dev"
)

func main() {
	options := []crane.Option{crane.WithAuthFromKeychain(gcrane.Keychain)}
	// Same as crane, but override usage and keychain.
	root := cmd.New(use, short, options)

	// Add or override commands.
	gcraneCmds := []*cobra.Command{gcmd.NewCmdList(), gcmd.NewCmdGc(), gcmd.NewCmdCopy(), cmd.NewCmdAuth(options, "gcrane", "auth")}

	// Maintain a map of google-specific commands that we "override".
	used := make(map[string]bool)
	for _, cmd := range gcraneCmds {
		used[cmd.Use] = true
	}

	// Remove those from crane's set of commands.
	for _, cmd := range root.Commands() {
		if _, ok := used[cmd.Use]; ok {
			root.RemoveCommand(cmd)
		}
	}

	// Add our own.
	for _, cmd := range gcraneCmds {
		root.AddCommand(cmd)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := root.ExecuteContext(ctx); err != nil {
		cancel()
		os.Exit(1)
	}
}
