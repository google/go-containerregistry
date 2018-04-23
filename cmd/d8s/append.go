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
	"flag"
	"log"
	"net/http"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/mutate"
	"github.com/google/go-containerregistry/v1/remote"
	"github.com/google/go-containerregistry/v1/tarball"
	"github.com/google/subcommands"
)

type appendCmd struct {
	outputFile string
}

func (*appendCmd) Name() string { return "append" }

func (*appendCmd) Synopsis() string {
	return "Appends a tarball to a remote image"
}
func (*appendCmd) Usage() string {
	return "append [-o output-file] <src-reference> <dest-tag> <tarball>"
}

func (a *appendCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&a.outputFile, "o", "", "output the resulting image to a new tarball")
}

func (a *appendCmd) Execute(
	ctx context.Context,
	f *flag.FlagSet,
	_ ...interface{}) subcommands.ExitStatus {

	if len(f.Args()) != 3 {
		return subcommands.ExitUsageError
	}

	src, dst, tar := f.Arg(0), f.Arg(1), f.Arg(2)

	srcRef, err := name.ParseReference(src, name.WeakValidation)
	if err != nil {
		log.Panicln(err)
	}

	srcAuth, err := authn.DefaultKeychain.Resolve(srcRef.Context().Registry)
	if err != nil {
		log.Panicln(err)
	}

	srcImage, err := remote.Image(srcRef, srcAuth, http.DefaultTransport)

	if err != nil {
		log.Panicln(err)
	}

	dstTag, err := name.NewTag(dst, name.WeakValidation)
	if err != nil {
		log.Panicln(err)
	}

	layer, err := tarball.LayerFromFile(tar)
	if err != nil {
		log.Panicln(err)
	}

	image, err := mutate.AppendLayers(srcImage, layer)
	if err != nil {
		log.Panicln(err)
	}

	if a.outputFile != "" {
		writeTarball(a.outputFile, dstTag, image)
		return subcommands.ExitSuccess
	}

	opts := remote.WriteOptions{}
	if srcRef.Context().RegistryStr() == dstTag.Context().RegistryStr() {
		opts.MountPaths = append(opts.MountPaths, srcRef.Context())
	}

	writeRemote(dstTag, image, opts)
	return subcommands.ExitSuccess
}

func writeRemote(ref name.Reference, i v1.Image, opts remote.WriteOptions) {
	dstAuth, err := authn.DefaultKeychain.Resolve(ref.Context().Registry)
	if err != nil {
		log.Panicln(err)
	}

	if err := remote.Write(ref, i, dstAuth, http.DefaultTransport, opts); err != nil {
		log.Panicln(err)
	}
}

func writeTarball(file string, tag name.Tag, i v1.Image) {
	if err := tarball.Write(file, tag, i, &tarball.WriteOptions{}); err != nil {
		log.Panicln(err)
	}
}
