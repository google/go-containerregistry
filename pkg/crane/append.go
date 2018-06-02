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
	"log"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func init() { Root.AddCommand(NewCmdAppend()) }

func NewCmdAppend() *cobra.Command {
	var output string
	appendCmd := &cobra.Command{
		Use:   "append",
		Short: "Append contents of a tarball to a remote image",
		Args:  cobra.ExactArgs(3),
		Run: func(_ *cobra.Command, args []string) {
			src, dst, tar := args[0], args[1], args[2]
			doAppend(src, dst, tar, output)
		},
	}
	appendCmd.Flags().StringVarP(&output, "output", "o", "", "Path to new tarball of resulting image")
	return appendCmd
}

func doAppend(src, dst, tar, output string) {
	srcRef, err := name.ParseReference(src, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing reference %q: %v", src, err)
	}

	srcAuth, err := authn.DefaultKeychain.Resolve(srcRef.Context().Registry)
	if err != nil {
		log.Fatalf("getting creds for %q: %v", srcRef, err)
	}

	srcImage, err := remote.Image(srcRef, srcAuth, http.DefaultTransport)
	if err != nil {
		log.Fatalf("reading image %q: %v", srcRef, err)
	}

	dstTag, err := name.NewTag(dst, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing tag %q: %v", dst, err)
	}

	layer, err := tarball.LayerFromFile(tar)
	if err != nil {
		log.Fatalf("reading tar %q: %v", tar, err)
	}

	image, err := mutate.AppendLayers(srcImage, layer)
	if err != nil {
		log.Fatalf("appending layer: %v", err)
	}

	if output != "" {
		if err := tarball.WriteToFile(output, dstTag, image, &tarball.WriteOptions{}); err != nil {
			log.Fatalf("writing output %q: %v", output, err)
		}
		return
	}

	dstAuth, err := authn.DefaultKeychain.Resolve(dstTag.Context().Registry)
	if err != nil {
		log.Fatalf("getting creds for %q: %v", dstTag, err)
	}

	opts := remote.WriteOptions{}
	if err := remote.Write(dstTag, image, dstAuth, http.DefaultTransport, opts); err != nil {
		log.Fatalf("writing image %q: %v", dstTag, err)
	}
}
