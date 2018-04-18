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
	"fmt"
	"log"
	"net/http"

	"github.com/google/subcommands"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/remote"
)

func getImage(r string) (v1.Image, error) {
	ref, err := name.ParseReference(r, name.WeakValidation)
	if err != nil {
		return nil, err
	}
	auth, err := authn.DefaultKeychain.Resolve(ref.Context().Registry)
	if err != nil {
		return nil, err
	}
	return remote.Image(ref, auth, http.DefaultTransport)
}

type configCmd struct{}

func (*configCmd) Name() string             { return "config" }
func (*configCmd) Synopsis() string         { return "Prints the image's config" }
func (*configCmd) Usage() string            { return "config <reference>" }
func (*configCmd) SetFlags(f *flag.FlagSet) {}

func (*configCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 1 {
		return subcommands.ExitUsageError
	}

	i, err := getImage(f.Args()[0])
	if err != nil {
		log.Fatalln(err)
	}
	config, err := i.RawConfigFile()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(config))
	return subcommands.ExitSuccess
}

type digestCmd struct{}

func (*digestCmd) Name() string             { return "digest" }
func (*digestCmd) Synopsis() string         { return "Prints the image's digest" }
func (*digestCmd) Usage() string            { return "digest <reference>" }
func (*digestCmd) SetFlags(f *flag.FlagSet) {}

func (*digestCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 1 {
		return subcommands.ExitUsageError
	}

	i, err := getImage(f.Args()[0])
	if err != nil {
		log.Fatalln(err)
	}
	digest, err := i.Digest()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(digest.String())
	return subcommands.ExitSuccess
}

type manifestCmd struct{}

func (*manifestCmd) Name() string             { return "manifest" }
func (*manifestCmd) Synopsis() string         { return "Prints the image's manifest" }
func (*manifestCmd) Usage() string            { return "manifest <reference>" }
func (*manifestCmd) SetFlags(f *flag.FlagSet) {}

func (*manifestCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 1 {
		return subcommands.ExitUsageError
	}

	i, err := getImage(f.Args()[0])
	if err != nil {
		log.Fatalln(err)
	}
	manifest, err := i.RawManifest()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(manifest))
	return subcommands.ExitSuccess
}
