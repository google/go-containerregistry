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
	untagged := true
	tags := []string{}
	before := ""
	after := ""

	recursive := false
	remove := false

	cmd := &cobra.Command{
		Use:   "gc",
		Short: "List images that are not tagged",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			opts, err := parseOpts(tags, before, after, untagged, recursive, remove)
			if err != nil {
				log.Fatalln(err)
			}
			GarbageCollect(args[0], opts)
		},
	}

	// Filters
	cmd.Flags().BoolVarP(&untagged, "untagged", "u", true, "Match only untagged images")
	cmd.Flags().StringSliceVarP(&tags, "tags", "t", []string{}, "Match images with these tags")
	cmd.Flags().StringVarP(&before, "before", "b", "", "Match images uploaded before this time (RFC3339)")
	cmd.Flags().StringVarP(&after, "after", "a", "", "Match images uploaded after this time (RFC3339)")

	// Behaviors
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Whether to recurse through repos")
	cmd.Flags().BoolVarP(&remove, "delete", "D", false, "Delete images instead of just printing them")

	return cmd
}

type GCOptions struct {
	Untagged bool
	Tags     []string

	// Set version of Tags, for convenience.
	tagSet map[string]struct{}

	Before *time.Time
	After  *time.Time

	Recursive bool
	Delete    bool
}

func GarbageCollect(root string, opts GCOptions) {
	repo, err := name.NewRepository(root, name.WeakValidation)
	if err != nil {
		log.Fatalln(err)
	}

	auth := google.WithAuthFromKeychain(authn.DefaultKeychain)

	// Turn the Tags list into a string set.
	opts.tagSet = make(map[string]struct{})
	for _, tag := range opts.Tags {
		opts.tagSet[tag] = struct{}{}
	}

	if opts.Recursive {
		if err := google.Walk(repo, opts.walkFn, auth); err != nil {
			log.Fatalln(err)
		}
		return
	}

	tags, err := google.List(repo, auth)
	if err := opts.walkFn(repo, tags, err); err != nil {
		log.Fatalln(err)
	}
}

func (o *GCOptions) getMatchingRefs(repo name.Repository, digest string, manifest google.ManifestInfo) ([]name.Reference, error) {
	// Evaluate all the filters for this image.
	untagged := len(manifest.Tags) == 0
	before := o.Before == nil || o.Before.After(manifest.Uploaded)
	after := o.After == nil || o.After.Before(manifest.Uploaded)

	matchesTags := false
	if len(o.tagSet) != 0 {
		for _, tag := range manifest.Tags {
			if _, ok := o.tagSet[tag]; ok {
				matchesTags = true
				break
			}
		}
	}

	refs := []name.Reference{}

	// Determine if this image matches the filters.
	if ((o.Untagged && untagged) || matchesTags) && before && after {
		// Add tags first if they need to be deleted.
		if !o.Untagged {
			for _, tag := range manifest.Tags {
				tagRef, err := name.ParseReference(fmt.Sprintf("%s:%s", repo, tag), name.StrictValidation)
				if err != nil {
					// Not expected since this is gcr.io specific.
					return nil, err
				}
				refs = append(refs, tagRef)
			}
		}

		digestRef, err := name.ParseReference(fmt.Sprintf("%s@%s", repo, digest), name.StrictValidation)
		if err != nil {
			// Not expected since this is gcr.io specific.
			return nil, err
		}
		refs = append(refs, digestRef)
	}

	return refs, nil
}

func (o *GCOptions) walkFn(repo name.Repository, tags *google.Tags, err error) error {
	if err != nil {
		return err
	}

	// Just get creds once per repo (should be once per invocation).
	auth, err := authn.DefaultKeychain.Resolve(repo.Registry)
	if err != nil {
		return fmt.Errorf("getting creds for %q: %v", repo, err)
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
					return fmt.Errorf("deleting image %q: %v", ref, err)
				}
			} else {
				fmt.Printf("%s\n", ref)
			}
		}
	}

	return nil
}

func parseOpts(tags []string, before, after string, untagged, recursive, remove bool) (GCOptions, error) {
	if len(tags) != 0 {
		// Overrides untagged
		untagged = false
	}

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
		Untagged:  untagged,
		Tags:      tags,
		After:     a,
		Before:    b,
	}, nil
}
