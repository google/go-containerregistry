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

	"github.com/google/subcommands"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1/remote"
)

type copyCmd struct{}

func (*copyCmd) Name() string { return "copy" }
func (*copyCmd) Synopsis() string {
	return "Efficiently copies a remote image from src reference to dst reference"
}
func (*copyCmd) Usage() string            { return "copy <src reference> <dst reference>" }
func (*copyCmd) SetFlags(f *flag.FlagSet) {}

func (*copyCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 2 {
		return subcommands.ExitUsageError
	}

	src, err := name.ParseReference(f.Args()[0], name.WeakValidation)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Pulling %v", src)

	srcAuth, err := authn.DefaultKeychain.Resolve(src.Context().Registry)
	if err != nil {
		log.Fatalln(err)
	}

	img, err := remote.Image(src, srcAuth, http.DefaultTransport)
	if err != nil {
		log.Fatalln(err)
	}

	dst, err := name.ParseReference(f.Args()[1], name.WeakValidation)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Pushing %v", dst)

	dstAuth, err := authn.DefaultKeychain.Resolve(dst.Context().Registry)
	if err != nil {
		log.Fatalln(err)
	}

	wo := remote.WriteOptions{
		MountPaths: []name.Repository{src.Context()},
	}

	if err := remote.Write(dst, img, dstAuth, http.DefaultTransport, wo); err != nil {
		log.Fatalln(err)
	}

	return subcommands.ExitSuccess
}
