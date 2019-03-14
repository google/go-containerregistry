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
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/cobra"
)

// Tag applied to images that were pulled by digest. This denotes that the
// image was (probably) never tagged with this, but lets us avoid applying the
// ":latest" tag which might be misleading.
const iWasADigestTag = "i-was-a-digest"

func init() { Root.AddCommand(NewCmdPull()) }

// NewCmdPull creates a new cobra.Command for the pull subcommand.
func NewCmdPull() *cobra.Command {
	allTags := false
	oci := false
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull a remote image by reference and store its contents in a tarball",
		Args:  cobra.ExactArgs(2),
		Run: func(cc *cobra.Command, args []string) {
			pull(args[0], args[1], allTags, oci)
		},
	}

	cmd.Flags().BoolVarP(&allTags, "all-tags", "a", false, "Whether to pull all tags in a repo")
	cmd.Flags().BoolVar(&oci, "oci", false, "Whether to write images as OCI image layout")
	return cmd
}

// TODO: better options.
func pull(src, dst string, allTags, oci bool) {
	if allTags && oci {
		if err := pullLayout(src, dst); err != nil {
			log.Fatal(err)
		}
		return
	}

	// TODO: Make allTags work for tarball
	// TODO: Make oci work without allTags
	ref, err := name.ParseReference(src, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing tag %q: %v", src, err)
	}
	log.Printf("Pulling %v", ref)

	i, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		log.Fatalf("reading image %q: %v", ref, err)
	}

	// WriteToFile wants a tag to write to the tarball, but we might have
	// been given a digest.
	// If the original ref was a tag, use that. Otherwise, if it was a
	// digest, tag the image with :i-was-a-digest instead.
	tag, ok := ref.(name.Tag)
	if !ok {
		d, ok := ref.(name.Digest)
		if !ok {
			log.Fatal("ref wasn't a tag or digest")
		}
		s := fmt.Sprintf("%s:%s", d.Repository.Name(), iWasADigestTag)
		tag, err = name.NewTag(s, name.WeakValidation)
		if err != nil {
			log.Fatalf("parsing digest as tag (%s): %v", s, err)
		}
	}

	if err := tarball.WriteToFile(dst, tag, i); err != nil {
		log.Fatalf("writing image %q: %v", dst, err)
	}
}

func pullLayout(src, dst string) error {
	// Open the layout.
	path, err := layout.FromPath(dst)
	if err != nil {
		// TODO: This could be cleaner. We can have an entrypoint that inits the layout
		// if it does not already exist.
		path, err = layout.Write(dst, empty.Index)
		if err != nil {
			return fmt.Errorf("cannot init layout: %v", err)
		}
	}

	// Fetch all the tags from "src" repo.
	repo, err := name.NewRepository(src, name.WeakValidation)
	if err != nil {
		return err
	}

	auth, err := authn.DefaultKeychain.Resolve(repo.Registry)
	if err != nil {
		return fmt.Errorf("getting creds for %q: %v", repo, err)
	}

	tags, err := remote.List(repo, auth, http.DefaultTransport)
	if err != nil {
		return fmt.Errorf("reading tags for %q: %v", repo, err)
	}

	// For each tag, append the image to the layout with an annotation describing where we pulled it from.
	// This will be useful when we push this layout to a new repo, since we'll want to reuse the tags.
	//
	// TODO: We could possibly dedupe the descriptors so that one manifest could have multiple tags,
	// but it's fine to have multiple entries for the same image in here, since re-pushing it will be a NOP.
	for _, tag := range tags {
		ref, err := name.ParseReference(fmt.Sprintf("%s:%s", repo, tag), name.WeakValidation)
		if err != nil {
			return err
		}
		annotation := layout.WithAnnotations(map[string]string{
			"org.opencontainers.image.ref.name": ref.String(),
		})
		auth := remote.WithAuthFromKeychain(authn.DefaultKeychain)

		// TODO: We can handle imageindex better once my remote.Descriptor concept lands.
		// For now, we just do the fallback thing. try in
		if err := appendIndex(path, ref, auth, annotation); err != nil {
			if err := appendImage(path, ref, auth, annotation); err != nil {
				return fmt.Errorf("appending %v: %v", ref, err)
			}
		}
	}
	return nil
}

func appendIndex(path layout.Path, ref name.Reference, auth remote.ImageOption, annotation layout.Option) error {
	idx, err := remote.Index(ref, auth)
	if err != nil {
		return fmt.Errorf("pulling %v: %v", ref, err)
	}
	return path.AppendIndex(idx, annotation)
}

func appendImage(path layout.Path, ref name.Reference, auth remote.ImageOption, annotation layout.Option) error {
	img, err := remote.Image(ref, auth)
	if err != nil {
		return fmt.Errorf("pulling %v: %v", ref, err)
	}
	return path.AppendImage(img, annotation)
}
