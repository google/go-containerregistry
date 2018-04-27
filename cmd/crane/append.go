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
	"log"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1/mutate"
	"github.com/google/go-containerregistry/v1/remote"
	"github.com/google/go-containerregistry/v1/tarball"
)

func init() {
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
	rootCmd.AddCommand(appendCmd)
}

func doAppend(src, dst, tar, output string) {
	srcRef, err := name.ParseReference(src, name.WeakValidation)
	if err != nil {
		log.Fatalln(err)
	}

	srcAuth, err := authn.DefaultKeychain.Resolve(srcRef.Context().Registry)
	if err != nil {
		log.Fatalln(err)
	}

	srcImage, err := remote.Image(srcRef, srcAuth, http.DefaultTransport)

	if err != nil {
		log.Fatalln(err)
	}

	dstTag, err := name.NewTag(dst, name.WeakValidation)
	if err != nil {
		log.Fatalln(err)
	}

	layer, err := tarball.LayerFromFile(tar)
	if err != nil {
		log.Fatalln(err)
	}

	image, err := mutate.AppendLayers(srcImage, layer)
	if err != nil {
		log.Fatalln(err)
	}

	if output != "" {
		if err := tarball.Write(output, dstTag, image, &tarball.WriteOptions{}); err != nil {
			log.Fatalln(err)
		}
		return
	}

	opts := remote.WriteOptions{}
	if srcRef.Context().RegistryStr() == dstTag.Context().RegistryStr() {
		opts.MountPaths = append(opts.MountPaths, srcRef.Context())
	}

	dstAuth, err := authn.DefaultKeychain.Resolve(dstTag.Context().Registry)
	if err != nil {
		log.Fatalln(err)
	}

	if err := remote.Write(dstTag, image, dstAuth, http.DefaultTransport, opts); err != nil {
		log.Fatalln(err)
	}
}
