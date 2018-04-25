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
	"log"

	"github.com/spf13/cobra"
)

func init() {
	var ref string
	digestCmd := &cobra.Command{
		Use:   "digest",
		Short: "Get the digest of an image",
		Run: func(*cobra.Command, []string) {
			digest(ref)
		},
	}
	digestCmd.Flags().StringVarP(&ref, "ref", "", "", "Image reference to get")
	rootCmd.AddCommand(digestCmd)
}

func digest(ref string) {
	if ref == "" {
		log.Fatalln("Must provide --ref")
	}
	i, err := getImage(ref)
	if err != nil {
		log.Fatalln(err)
	}
	digest, err := i.Digest()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(digest.String())
}
