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
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var cmds = &cobra.Command{
	Use:   "crane",
	Short: "Crane is a tool for managing container images",
	Run:   func(cmd *cobra.Command, _ []string) { cmd.Usage() },
}

var gendoc = flag.String("gendoc", "", "If set, path to generate docs and exit")

func main() {
	flag.Parse()
	if *gendoc != "" {
		if err := doc.GenMarkdownTree(rootCmd, *gendoc); err != nil {
			log.Fatalln(err)
		}
		return
	}

	cmds.AddCommand(crane.NewCmdAppend())
	cmds.AddCommand(crane.NewCmdConfig())
	cmds.AddCommand(crane.NewCmdCopy())
	cmds.AddCommand(crane.NewCmdDelete())
	cmds.AddCommand(crane.NewCmdDigest())
	cmds.AddCommand(crane.NewCmdList())
	cmds.AddCommand(crane.NewCmdManifest())
	cmds.AddCommand(crane.NewCmdPull())
	cmds.AddCommand(crane.NewCmdPush())
	cmds.AddCommand(crane.NewCmdRebase())

	if err := cmds.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
