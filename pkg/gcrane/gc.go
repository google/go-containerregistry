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

package gcrane

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdGc()) }

func NewCmdGc() *cobra.Command {
	before := ""
	after := ""

	recursive := false
	remove := false
	untag := false

	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Garbage collect images",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			opts, err := parseOpts(before, after, recursive, remove, untag)
			if err != nil {
				log.Fatalln(err)
			}

			if err := GarbageCollect(args[0], opts); err != nil {
				log.Fatalln(err)
			}
		},
	}

	// Filters
	cmd.Flags().StringVarP(&before, "before", "b", "", "Match images uploaded before this time (RFC3339)")
	cmd.Flags().StringVarP(&after, "after", "a", "", "Match images uploaded after this time (RFC3339)")

	// Behaviors
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Whether to recurse through repos")
	cmd.Flags().BoolVarP(&remove, "delete", "D", false, "Delete images instead of just printing them")
	cmd.Flags().BoolVarP(&untag, "untag", "U", false, "Also untag matching images")

	return cmd
}

type GCOptions struct {
	Before *time.Time
	After  *time.Time

	Recursive bool
	Delete    bool
	Untag     bool
}

// GarbageCollect iterates over a repository, printing and optionally deleting
// images that match the criteria in opts.
func GarbageCollect(root string, opts GCOptions) error {
	repo, err := name.NewRepository(root, name.WeakValidation)
	if err != nil {
		return err
	}

	// TODO: Ideally we would just do the token handshake once (or not at all).
	auth := google.WithAuthFromKeychain(authn.DefaultKeychain)

	if opts.Recursive {
		return google.Walk(repo, opts.walkFn, auth)
	}

	tags, err := google.List(repo, auth)
	return opts.walkFn(repo, tags, err)
}

func (o *GCOptions) getMatchingRefs(repo name.Repository, digest string, manifest google.ManifestInfo) ([]name.Reference, error) {
	// Evaluate all the filters for this image.
	before := o.Before == nil || o.Before.After(manifest.Uploaded)
	after := o.After == nil || o.After.Before(manifest.Uploaded)

	refs := []name.Reference{}
	digestRef, err := name.ParseReference(fmt.Sprintf("%s@%s", repo, digest), name.StrictValidation)
	if err != nil {
		return nil, err
	}

	// Determine if this image matches the filters.
	if before && after {
		// If --untag is set, also delete any image tags.
		if o.Untag {
			for _, tag := range manifest.Tags {
				tagRef, err := name.ParseReference(fmt.Sprintf("%s:%s", repo, tag), name.StrictValidation)
				if err != nil {
					return nil, err
				}
				refs = append(refs, tagRef)
			}
		}

		// Don't bother adding the digest unless we can delete it.
		if o.Untag || len(manifest.Tags) == 0 {
			refs = append(refs, digestRef)
		} else if o.Delete {
			log.Printf("Skipping %s because it is pinned by tags: %v", digestRef, manifest.Tags)
		}
	}

	return refs, nil
}

func (o *GCOptions) walkFn(repo name.Repository, tags *google.Tags, err error) error {
	if err != nil {
		return err
	}

	// Just get creds once per repo.
	auth, err := authn.DefaultKeychain.Resolve(repo.Registry)
	if err != nil {
		return fmt.Errorf("error getting creds for %q: %v", repo, err)
	}

	for digest, manifest := range tags.Manifests {
		// Determine if this image (and its tags) matches our filters.
		refs, err := o.getMatchingRefs(repo, digest, manifest)
		if err != nil {
			return err
		}

		// Delete or print them.
		for _, ref := range refs {
			if o.Delete {
				log.Printf("Deleting %s", ref)
				if err := remote.Delete(ref, auth, http.DefaultTransport); err != nil {
					// Just log the error instead of failing.
					log.Printf("error deleting %q: %v", ref, err)
				}
			} else {
				fmt.Printf("%s\n", ref)
			}
		}
	}

	return nil
}

func parseOpts(before, after string, recursive, remove, untag bool) (GCOptions, error) {
	var a, b *time.Time

	if after != "" {
		at, err := time.Parse(time.RFC3339, after)
		if err != nil {
			return GCOptions{}, err
		}
		a = &at
	}

	if before != "" {
		bt, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return GCOptions{}, err
		}
		b = &bt
	}

	if a != nil && b != nil {
		if b.Before(*a) {
			return GCOptions{}, fmt.Errorf("There is no time both before %s and after %s.", b, a)
		}
	}

	return GCOptions{
		Recursive: recursive,
		Delete:    remove,
		Untag:     untag,
		After:     a,
		Before:    b,
	}, nil
}
