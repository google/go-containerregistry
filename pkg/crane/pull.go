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

	"github.com/spf13/cobra"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func init() { Root.AddCommand(NewCmdPull()) }

func NewCmdPull() *cobra.Command {
	return &cobra.Command{
		Use:   "pull",
		Short: "Pull a remote image by reference and store its contents in a tarball",
		Args:  cobra.ExactArgs(2),
		Run:   pull,
	}
}

func pull(_ *cobra.Command, args []string) {
	src, dst := args[0], args[1]
	// TODO: Why is only tag allowed?
	t, err := name.NewTag(src, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing tag %q: %v", src, err)
	}
	log.Printf("Pulling %v", t)

	i, err := remote.Image(t, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		log.Fatalf("reading image %q: %v", t, err)
	}

	if err := tarball.WriteToFile(dst, t, i, &tarball.WriteOptions{}); err != nil {
		log.Fatalf("writing image %q: %v", dst, err)
	}
}
