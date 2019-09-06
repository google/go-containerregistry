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
	"regexp"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdPurge()) }

var matchTag string
var skipTag string
var match *regexp.Regexp
var skip *regexp.Regexp
var dryRun bool
var concurrency int

type deleteRequest struct {
	repo   name.Repository
	digest string
	tags   []string
}

// NewCmdPurge creates a new cobra.Command for the purge subcommand.
func NewCmdPurge() *cobra.Command {
	recursive := false
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Removes images with a tag that matches a regex",
		Long:  "This will remove images that have a tag that matches the match regex and doesn't have a tag that matches the skip regex",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			purge(args[0], recursive, matchTag)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Whether to recurse through repos")
	cmd.Flags().StringVarP(&matchTag, "match", "m", "", "regular expression to match against tags")
	cmd.Flags().StringVarP(&skipTag, "skip", "s", "", "skip if regular expression matches against tags")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "only print images to purge, don't delete")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "c", 1, "concurrency for deleting")
	return cmd
}

func purge(root string, recursive bool, matchTag string) {
	if matchTag == "" {
		log.Fatal("match needs to be set")
	}
	match = regexp.MustCompile(matchTag)
	if skipTag != "" {
		skip = regexp.MustCompile(skipTag)
	}
	repo, err := name.NewRepository(root)
	if err != nil {
		log.Fatalln(err)
	}
	wg := &sync.WaitGroup{}
	wg.Add(concurrency)
	workC := make(chan deleteRequest, 1)
	auth, err := google.Keychain.Resolve(repo.Registry)
	if err != nil {
		log.Fatalf("getting auth for %q: %v", root, err)
	}
	for i := 0; i < concurrency; i++ {
		go deleteWorker(wg, workC, auth)
	}
	if recursive {
		if err := google.Walk(repo, purgeWalkFactory(workC, match, skip, dryRun), google.WithAuth(auth)); err != nil {
			log.Fatalln(err)
		}
		return
	}

	tags, err := google.List(repo, google.WithAuth(auth))
	if err := purgeWalkFactory(workC, match, skip, dryRun)(repo, tags, err); err != nil {
		log.Fatalln(err)
	}
	close(workC)
	wg.Wait()
}

// Delete deletes the remote reference at src.
func delete(src string, auth authn.Authenticator) error {
	ref, err := name.ParseReference(src)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", src, err)
	}

	return remote.Delete(ref, remote.WithAuth(auth))
}

// deleteWorker will process deleteRequests
func deleteWorker(wg *sync.WaitGroup, workC chan deleteRequest, auth authn.Authenticator) {
	defer wg.Done()
	for req := range workC {
		for _, tag := range req.tags {
			repoTag := fmt.Sprintf("%s:%s", req.repo, tag)
			err := delete(repoTag, auth)
			if err != nil {
				log.Printf("ERROR: unable to delete %s %s\n", repoTag)
			}
		}
		repoDigest := fmt.Sprintf("%s@%s", req.repo, req.digest)
		err := delete(repoDigest, auth)
		if err != nil {
			log.Printf("ERROR: unable to delete %s %s\n", repoDigest)
		} else {
			log.Printf("DELETED %s %v\n", repoDigest, req.tags)
		}
	}
}

// purgeWalkFactory returns a WalkFun that sends delete messages to workC
// TODO: this is akward, but needs to end up with a google.WalkFunc
func purgeWalkFactory(workC chan deleteRequest, match *regexp.Regexp, skip *regexp.Regexp, dryRun bool) google.WalkFunc {
	return func(repo name.Repository, tags *google.Tags, err error) error {
		if err != nil {
			return err
		}
		for digest, manifest := range tags.Manifests {
			// TODO: is there a cleaner way to do this?
			skipImage := false
			if skip != nil {
				for _, tag := range manifest.Tags {
					if skip.MatchString(tag) {
						skipImage = true
						break
					}
				}
			}
			repoDigest := fmt.Sprintf("%s@%s", repo, digest)
			if skipImage {
				log.Printf("SKIPPING: %s %v\n", repoDigest, manifest.Tags)
				continue
			}
			shouldPurge := false
			for _, tag := range manifest.Tags {
				if match.MatchString(tag) {
					if dryRun {
						log.Printf("DRY-RUN: %s %v\n", repoDigest, manifest.Tags)
					} else {
						shouldPurge = true
					}
					break
				}
			}
			if !shouldPurge {
				continue
			}
			req := deleteRequest{
				repo:   repo,
				tags:   manifest.Tags,
				digest: digest,
			}
			workC <- req
		}
		return nil
	}
}
