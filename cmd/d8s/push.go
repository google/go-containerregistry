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

type pushCmd struct{}

func (*pushCmd) Name() string { return "push" }
func (*pushCmd) Synopsis() string {
	return "Pushes image contents as a tarball to a remote registry"
}
func (*pushCmd) Usage() string            { return "push <tarball> <tag>" }
func (*pushCmd) SetFlags(f *flag.FlagSet) {}

func (*pushCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 2 {
		return subcommands.ExitUsageError
	}

	path, tag := f.Args()[0], f.Args()[1]
	t, err := name.NewTag(tag, name.WeakValidation)
	if err != nil {
		log.Panicln(err)
	}
	log.Printf("Pushing %v", t)

	auth, err := authn.DefaultKeychain.Resolve(t.Registry)
	if err != nil {
		log.Panicln(err)
	}

	i, err := tarball.ImageFromPath(path, nil)
	if err != nil {
		log.Panicln(err)
	}

	if err := remote.Write(t, i, auth, http.DefaultTransport, remote.WriteOptions{}); err != nil {
		log.Panicln(err)
	}
	return subcommands.ExitSuccess
}
