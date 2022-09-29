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
	"errors"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/cobra"
)

// NewCmdAttach creates a new cobra.Command for the attach subcommand.
func NewCmdAttach(options *[]crane.Option) *cobra.Command {
	var attachType, fn string

	attachmentsCmd := &cobra.Command{
		Use:   "attach",
		Short: "Add an attachment to an image",
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
					return err
				}
				dig = ref.Context().Digest(desc.Digest.String())
			}

			desc, err := remote.Head(dig) // TODO options
			var terr *transport.Error
			if errors.As(err, &terr) && terr.StatusCode == http.StatusNotFound {
				h, err := v1.NewHash(dig.DigestStr())
				if err != nil {
					return err
				}
				// The subject doesn't exist, attach to it as if it's an empty image.
				desc = &v1.Descriptor{
					MediaType: types.OCIManifestSchema1,
					Size:      0,
					Digest:    h,
				}
			} else if err != nil {
				return err
			}

			att := mutate.Subject(empty.Image, *desc).(v1.Image)
			b, err := os.ReadFile(fn)
			if err != nil {
				return err
			}
			att, err = mutate.AppendLayers(att, static.NewLayer(b, types.MediaType(attachType)))
			if err != nil {
				return err
			}
			attdig, err := att.Digest()
			if err != nil {
				return err
			}
			attref := ref.Context().Digest(attdig.String())
			return remote.Write(attref, att) // TODO options
		},
	}
	attachmentsCmd.Flags().StringVarP(&attachType, "type", "t", "", "Type of attachment")
	attachmentsCmd.Flags().StringVarP(&fn, "file", "f", "", "Name of file to attach")
	attachmentsCmd.MarkFlagRequired("type")
	attachmentsCmd.MarkFlagRequired("file")
	return attachmentsCmd
}
