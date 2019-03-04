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
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdCopy()) }

// NewCmdCopy creates a new cobra.Command for the copy subcommand.
func NewCmdCopy() *cobra.Command {
	return &cobra.Command{
		Use:     "copy",
		Aliases: []string{"cp"},
		Short:   "Efficiently copy a remote image from src to dst",
		Args:    cobra.ExactArgs(2),
		Run:     doCopy,
	}
}

func doCopy(_ *cobra.Command, args []string) {
	src, dst := args[0], args[1]
	srcRef, err := name.ParseReference(src, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing reference %q: %v", src, err)
	}
	log.Printf("Pulling %v", srcRef)

	dstRef, err := name.ParseReference(dst, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing reference %q: %v", dst, err)
	}
	log.Printf("Pushing %v", dstRef)

	srcAuth, err := authn.DefaultKeychain.Resolve(srcRef.Context().Registry)
	if err != nil {
		log.Fatalf("getting creds for %q: %v", srcRef, err)
	}

	dstAuth, err := authn.DefaultKeychain.Resolve(dstRef.Context().Registry)
	if err != nil {
		log.Fatalf("getting creds for %q: %v", dstRef, err)
	}

	// First, try to copy as an index.
	// If that fails, try to copy as an image.
	// We have to try this second because fallback logic exists in the registry
	// to convert an index to an image.
	// TODO(#388): Figure out which artifact is returned at runtime.
	if err := copyIndex(srcRef, dstRef, srcAuth, dstAuth); err != nil {
		if err := copyImage(srcRef, dstRef, srcAuth, dstAuth); err != nil {
			log.Fatalf("failed to copy image: %v", err)
		}
	}
}

func copyImage(srcRef, dstRef name.Reference, srcAuth, dstAuth authn.Authenticator) error {
	img, err := remote.Image(srcRef, remote.WithAuth(srcAuth))
	if err != nil {
		return fmt.Errorf("reading image %q: %v", srcRef, err)
	}

	if err := remote.Write(dstRef, img, dstAuth, http.DefaultTransport); err != nil {
		return fmt.Errorf("writing image %q: %v", dstRef, err)
	}

	return nil
}

func copyIndex(srcRef, dstRef name.Reference, srcAuth, dstAuth authn.Authenticator) error {
	idx, err := remote.Index(srcRef, remote.WithAuth(srcAuth))
	if err != nil {
		return fmt.Errorf("reading index %q: %v", srcRef, err)
	}

	if err := remote.WriteIndex(dstRef, idx, dstAuth, http.DefaultTransport); err != nil {
		return fmt.Errorf("writing index %q: %v", dstRef, err)
	}

	return nil
}
