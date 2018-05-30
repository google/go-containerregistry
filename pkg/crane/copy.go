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

	"net/http"

	"github.com/google/go-containerregistry/v1/remote"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/spf13/cobra"
)

func NewCmdCopy() *cobra.Command {
	return &cobra.Command{
		Use:   "copy",
		Short: "Efficiently copy a remote image from src to dst",
		Args:  cobra.ExactArgs(2),
		Run:   doCopy,
	}
}

func doCopy(_ *cobra.Command, args []string) {
	src, dst := args[0], args[1]
	srcRef, err := name.ParseReference(src, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing reference %q: %v", src, err)
	}
	log.Printf("Pulling %v", srcRef)

	srcAuth, err := authn.DefaultKeychain.Resolve(srcRef.Context().Registry)
	if err != nil {
		log.Fatalf("getting creds for %q: %v", srcRef, err)
	}

	img, err := remote.Image(srcRef, srcAuth, http.DefaultTransport, nil)
	if err != nil {
		log.Fatalf("reading image %q: %v", srcRef, err)
	}

	dstRef, err := name.ParseReference(dst, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing reference %q: %v", dst, err)
	}
	log.Printf("Pushing %v", dstRef)

	dstAuth, err := authn.DefaultKeychain.Resolve(dstRef.Context().Registry)
	if err != nil {
		log.Fatalf("getting creds for %q: %v", dstRef, err)
	}

	wo := remote.WriteOptions{
		MountPaths: []name.Repository{srcRef.Context()},
	}

	if err := remote.Write(dstRef, img, dstAuth, http.DefaultTransport, wo); err != nil {
		log.Fatalf("writing image %q: %v", dstRef, err)
	}
}
