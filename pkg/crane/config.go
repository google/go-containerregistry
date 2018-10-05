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
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdConfig()) }

func NewCmdConfig() *cobra.Command {
	var pretty bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Get the config of an image",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			config(args[0], pretty)
		},
	}
	cmd.Flags().BoolVarP(&pretty, "pretty", "p", false, "If true, pretty-print JSON")
	return cmd
}

func config(ref string, pretty bool) {
	i, _, err := getImage(ref)
	if err != nil {
		log.Fatalln(err)
	}
	if !pretty {
		config, err := i.RawConfigFile()
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Print(string(config))
	} else {
		config, err := i.ConfigFile()
		if err != nil {
			log.Fatalln(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(config)
	}
}
