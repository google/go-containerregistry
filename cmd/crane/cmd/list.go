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
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdList()) }

// NewCmdList creates a new cobra.Command for the ls subcommand.
func NewCmdList() *cobra.Command {
	var all bool
	listCmd := &cobra.Command{
		Use:   "ls REPO",
		Short: "List the tags in a repo",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			repo := args[0]
			if all {
				_, tagDetailsList, err := crane.ListTagsDetails(repo, options...)
				if err != nil {
					log.Fatalf("reading tags for %s: %v", repo, err)
				}

				// sort the tag details list by creation time
				sort.Slice(tagDetailsList, func(i, j int) bool {
					return tagDetailsList[i].CreateTime.After(tagDetailsList[j].CreateTime)
				})

				tw := tabwriter.NewWriter(os.Stdout, 30, 8, 2, '\t', 0)
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
					"TAGS",
					"SHA",
					"CREATE TIME",
					"UPLOAD TIME",
				)
				for _, tag := range tagDetailsList {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
						strings.Join(tag.Tags, ","),
						tag.Sha,
						tag.CreateTime.Format(time.RFC822),
						tag.UploadTime.Format(time.RFC822),
					)
					tw.Flush()
				}
			} else {
				tags, err := crane.ListTags(repo, options...)
				if err != nil {
					log.Fatalf("reading tags for %s: %v", repo, err)
				}
				for _, tag := range tags {
					fmt.Println(tag)
				}
			}
		},
	}

	listCmd.Flags().BoolVarP(&all, "all", "a", false, "List all tag details")

	return listCmd
}
