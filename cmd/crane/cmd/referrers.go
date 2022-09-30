// Copyright 2022 Google LLC All Rights Reserved.
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
	"log"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
)

// NewCmdReferrers creates a new cobra.Command for the referrers subcommand.
func NewCmdReferrers(options *[]crane.Option) *cobra.Command {
	referrersCmd := &cobra.Command{
		Use:   "referrers",
		Short: "List referrers of an image",
		// TODO: Long
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			refstr := args[0]

			var dig name.Digest
			ref, err := name.ParseReference(refstr)
			if err != nil {
				return err
			}
			if digr, ok := ref.(name.Digest); ok {
				dig = digr
			} else {
				desc, err := remote.Head(ref) // TODO options
				if err != nil {
					// If you asked for a tag and it doesn't exist, we can't help you.
					return err
				}
				dig = ref.Context().Digest(desc.Digest.String())
			}

			descs, err := remote.Referrers(dig) // TODO options
			if err != nil {
				return err
			}
			for _, d := range descs {
				log.Println("-", d.Digest, d.MediaType) // TODO: format for real
			}
			return nil
		},
	}
	return referrersCmd
}
