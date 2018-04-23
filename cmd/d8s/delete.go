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
	"github.com/google/go-containerregistry/v1/remote"
	"github.com/google/subcommands"
)

type deleteCmd struct{}

func (*deleteCmd) Name() string             { return "delete" }
func (*deleteCmd) Synopsis() string         { return "Deletes a reference from its registry" }
func (*deleteCmd) Usage() string            { return "delete <reference>" }
func (*deleteCmd) SetFlags(f *flag.FlagSet) {}

func (*deleteCmd) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 1 {
		return subcommands.ExitUsageError
	}
	ref := f.Args()[0]

	r, err := name.ParseReference(ref, name.WeakValidation)
	if err != nil {
		log.Panicln(err)
	}

	auth, err := authn.DefaultKeychain.Resolve(r.Context().Registry)
	if err != nil {
		log.Panicln(err)
	}

	if err := remote.Delete(r, auth, http.DefaultTransport, remote.DeleteOptions{}); err != nil {
		log.Panicln(err)
	}
	return subcommands.ExitSuccess
}
