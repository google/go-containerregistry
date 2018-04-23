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
	"github.com/google/go-containerregistry/v1/tarball"
)

type pullCmd struct{}

func (*pullCmd) Name() string { return "pull" }
func (*pullCmd) Synopsis() string {
	return "Pulls an image by reference and stores its contents in a tarball"
}
func (*pullCmd) Usage() string            { return "pull <reference> <tarball>" }
func (*pullCmd) SetFlags(f *flag.FlagSet) {}

func (*pullCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 2 {
		return subcommands.ExitUsageError
	}

	tag, path := f.Args()[0], f.Args()[1]
	t, err := name.NewTag(tag, name.WeakValidation)
	if err != nil {
		log.Panicln(err)
	}
	log.Printf("Pulling %v", t)

	auth, err := authn.DefaultKeychain.Resolve(t.Registry)
	if err != nil {
		log.Panicln(err)
	}

	i, err := remote.Image(t, auth, http.DefaultTransport)
	if err != nil {
		log.Panicln(err)
	}

	if err := tarball.Write(path, t, i, &tarball.WriteOptions{}); err != nil {
		log.Panicln(err)
	}
	return subcommands.ExitSuccess
}
