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

package main

import (
	"context"
	"log"
	"os"

	"github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/google/go-containerregistry/internal/signal"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
)

func init() {
	logs.Warn.SetOutput(os.Stderr)
	logs.Progress.SetOutput(os.Stderr)
}

const (
	use   = "krane"
	short = "krane is a tool for managing container images"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	kc, err := k8schain.NewNoClient(ctx)
	if err != nil {
		log.Fatalf("Unable to create kubernetes-style keychain: %v", err)
	}
	keychain := authn.NewMultiKeychain(authn.DefaultKeychain, kc)

	// Same as crane, but override usage and keychain.
	root := cmd.New(use, short, []crane.Option{crane.WithAuthFromKeychain(keychain)})

	if err := root.ExecuteContext(ctx); err != nil {
		cancel()
		os.Exit(1)
	}
}
