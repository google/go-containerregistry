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
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/cobra"
)

const ociNameAnnotation = "org.opencontainers.image.ref.name"

func init() { Root.AddCommand(NewCmdPush()) }

// NewCmdPush creates a new cobra.Command for the push subcommand.
func NewCmdPush() *cobra.Command {
	allTags := false
	oci := false
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push image contents as a tarball to a remote registry",
		Args:  cobra.ExactArgs(2),
		Run: func(cc *cobra.Command, args []string) {
			push(args[0], args[1], allTags, oci)
		},
	}

	cmd.Flags().BoolVarP(&allTags, "all-tags", "a", false, "Whether to pull all tags in a repo")
	cmd.Flags().BoolVar(&oci, "oci", false, "Whether to write images as OCI image layout")
	return cmd
}

func push(src, dst string, allTags, oci bool) {
	if allTags && oci {
		if err := pushLayout(src, dst); err != nil {
			log.Fatal(err)
		}
		return
	}

	t, err := name.NewTag(dst, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing tag %q: %v", dst, err)
	}
	log.Printf("Pushing %v", t)

	auth, err := authn.DefaultKeychain.Resolve(t.Registry)
	if err != nil {
		log.Fatalf("getting creds for %q: %v", t, err)
	}

	i, err := tarball.ImageFromPath(src, nil)
	if err != nil {
		log.Fatalf("reading image %q: %v", src, err)
	}

	if err := remote.Write(t, i, auth, http.DefaultTransport); err != nil {
		log.Fatalf("writing image %q: %v", t, err)
	}
}

func renameImage(dst, refName string) (name.Reference, error) {
	srcTag, err := name.NewTag(refName, name.WeakValidation)
	if err != nil {
		return nil, err
	}
	tag := srcTag.Identifier()
	return name.NewTag(fmt.Sprintf("%s:%s", dst, tag), name.WeakValidation)
}

func pushLayout(src, dst string) error {
	// Open the layout.
	ii, err := layout.ImageIndexFromPath(src)
	if err != nil {
		return fmt.Errorf("cannot open layout: %v", err)
	}

	index, err := ii.IndexManifest()
	if err != nil {
		return err
	}

	// For each manifest referenced by the layout, we'll parse out the source
	// from the annotations and use the same tag when we push to the destination.
	for _, desc := range index.Manifests {
		// TODO: We could just upload by digest here.
		refName, ok := desc.Annotations[ociNameAnnotation]
		if !ok {
			return fmt.Errorf("expected %q annotation", ociNameAnnotation)
		}

		// Reswizzle the source image ref to be the "dst" repo + the source tag.
		ref, err := renameImage(dst, refName)
		if err != nil {
			return fmt.Errorf("couldn't rename ref: %v", err)
		}

		auth, err := authn.DefaultKeychain.Resolve(ref.Context().Registry)
		if err != nil {
			return fmt.Errorf("failed to auth %v: %v", ref, err)
		}

		switch desc.MediaType {
		case types.OCIImageIndex, types.DockerManifestList:
			ii, err := ii.ImageIndex(desc.Digest)
			if err != nil {
				return err
			}

			if err := remote.WriteIndex(ref, ii, auth, http.DefaultTransport); err != nil {
				return err
			}
		case types.OCIManifestSchema1, types.DockerManifestSchema2:
			img, err := ii.Image(desc.Digest)
			if err != nil {
				return err
			}
			if err := remote.Write(ref, img, auth, http.DefaultTransport); err != nil {
				return err
			}
		}
	}

	return nil
}
