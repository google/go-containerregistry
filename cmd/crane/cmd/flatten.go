// Copyright 2021 Google LLC All Rights Reserved.
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
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/spf13/cobra"
)

// NewCmdFlatten creates a new cobra.Command for the flatten subcommand.
func NewCmdFlatten(options *[]crane.Option) *cobra.Command {
	var newRef string

	flattenCmd := &cobra.Command{
		Use:   "flatten",
		Short: "Flatten an image's layers into a single layer",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Pull image and get config.
			ref := args[0]

			// If the new ref isn't provided, write over the original image.
			// If that ref was provided by digest (e.g., output from
			// another crane command), then strip that and push the
			// mutated image by digest instead.
			if newRef == "" {
				newRef = ref
			}

			// Stupid hack to support insecure flag.
			nameOpt := []name.Option{}
			if ok, err := cmd.Parent().PersistentFlags().GetBool("insecure"); err != nil {
				log.Fatalf("flag problems: %v", err)
			} else if ok {
				nameOpt = append(nameOpt, name.Insecure)
			}
			r, err := name.ParseReference(newRef, nameOpt...)
			if err != nil {
				log.Fatalf("parsing %s: %v", newRef, err)
			}

			desc, err := crane.Head(ref, *options...)
			if err != nil {
				log.Fatalf("checking %s: %v", ref, err)
			}
			if !cmd.Parent().PersistentFlags().Changed("platform") && desc.MediaType.IsIndex() {
				log.Fatalf("flattening an index is not yet supported")
			}

			old, err := crane.Pull(ref, *options...)
			if err != nil {
				log.Fatalf("pulling %s: %v", ref, err)
			}

			m, err := old.Manifest()
			if err != nil {
				log.Fatalf("reading manifest: %v", err)
			}

			cf, err := old.ConfigFile()
			if err != nil {
				log.Fatalf("getting config: %v", err)
			}
			cf = cf.DeepCopy()

			oldHistory, err := json.Marshal(cf.History)
			if err != nil {
				log.Fatalf("marshal history")
			}

			// Clear layer-specific config file information.
			cf.RootFS.DiffIDs = []v1.Hash{}
			cf.History = []v1.History{}

			img, err := mutate.ConfigFile(empty.Image, cf)
			if err != nil {
				log.Fatalf("mutating config: %v", err)
			}

			// TODO: Make compression configurable?
			layer := stream.NewLayer(mutate.Extract(old), stream.WithCompressionLevel(gzip.BestCompression))

			img, err = mutate.Append(img, mutate.Addendum{
				Layer: layer,
				History: v1.History{
					CreatedBy: fmt.Sprintf("crane flatten %s", ref),
					Comment:   string(oldHistory),
				},
			})
			if err != nil {
				log.Fatalf("appending layers: %v", err)
			}

			// Retain any annotations from the original image.
			if len(m.Annotations) != 0 {
				img = mutate.Annotations(img, m.Annotations).(v1.Image)
			}

			if _, ok := r.(name.Digest); ok {
				// If we're pushing by digest, we need to upload the layer first.
				if err := crane.Upload(layer, r.Context().String(), *options...); err != nil {
					log.Fatalf("uploading layer: %v", err)
				}
				digest, err := img.Digest()
				if err != nil {
					log.Fatalf("digesting new image: %v", err)
				}
				newRef = r.Context().Digest(digest.String()).String()
			}
			if err := crane.Push(img, newRef, *options...); err != nil {
				log.Fatalf("pushing %s: %v", newRef, err)
			}
			digest, err := img.Digest()
			if err != nil {
				log.Fatalf("digesting new image: %v", err)
			}
			fmt.Println(r.Context().Digest(digest.String()))
		},
	}
	flattenCmd.Flags().StringVarP(&newRef, "tag", "t", "", "New tag to apply to flattened image. If not provided, push by digest to the original image repository.")
	return flattenCmd
}
