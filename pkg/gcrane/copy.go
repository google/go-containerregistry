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
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/internal/retry"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"golang.org/x/sync/errgroup"
)

// Keychain tries to use google-specific credential sources, falling back to
// the DefaultKeychain (config-file based).
var Keychain = authn.NewMultiKeychain(google.Keychain, authn.DefaultKeychain)

// GCRBackoff returns a retry.Backoff that is suitable for use with gcr.io.
//
// These numbers are based on GCR's posted quotas:
// https://cloud.google.com/container-registry/quotas
// -  30k requests per 10 minutes.
// - 500k requests per 24 hours.
//
// On error, we will wait for:
// - 6 seconds (in case of very short term 429s from GCS), then
// - 1 minute (in case of temporary network issues), then
// - 10 minutes (to get around GCR 10 minute quotas), then fail.
//
// TODO: In theory, we could keep retrying until the next day to get around the 500k limit.
func GCRBackoff() retry.Backoff {
	return retry.Backoff{
		Duration: 6 * time.Second,
		Factor:   10.0,
		Jitter:   0.1,
		Steps:    3,
		Cap:      1 * time.Hour,
	}
}

// Copy copies a remote image or index from src to dst.
func Copy(src, dst string, opts ...Option) error {
	o := makeOptions(opts...)
	// Just reuse crane's copy logic with gcrane's credential logic.
	return crane.Copy(src, dst, o.crane...)
}

// CopyRepository copies everything from the src GCR repository to the
// dst GCR repository.
func CopyRepository(ctx context.Context, src, dst string, opts ...Option) error {
	o := makeOptions(opts...)
	return recursiveCopy(ctx, src, dst, o)
}

type task struct {
	digest   string
	manifest google.ManifestInfo
	oldRepo  name.Repository
	newRepo  name.Repository
}

type copier struct {
	srcRepo name.Repository
	dstRepo name.Repository

	tasks chan task
	opt   *options
}

func newCopier(src, dst string, o *options) (*copier, error) {
	srcRepo, err := name.NewRepository(src)
	if err != nil {
		return nil, fmt.Errorf("parsing repo %q: %w", src, err)
	}

	dstRepo, err := name.NewRepository(dst)
	if err != nil {
		return nil, fmt.Errorf("parsing repo %q: %w", dst, err)
	}

	// A queue of size 2*jobs should keep each goroutine busy.
	tasks := make(chan task, o.jobs*2)

	return &copier{srcRepo, dstRepo, tasks, o}, nil
}

// recursiveCopy copies images from repo src to repo dst.
func recursiveCopy(ctx context.Context, src, dst string, o *options) error {
	c, err := newCopier(src, dst, o)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)
	walkFn := func(repo name.Repository, tags *google.Tags, err error) error {
		if err != nil {
			logs.Warn.Printf("failed walkFn for repo %s: %v", repo, err)
			// If we hit an error when listing the repo, try re-listing with backoff.
			if err := backoffErrors(GCRBackoff(), func() error {
				tags, err = google.List(repo, o.google...)
				return err
			}); err != nil {
				return fmt.Errorf("failed List for repo %s: %w", repo, err)
			}
		}

		// If we hit an error when trying to diff the repo, re-diff with backoff.
		if err := backoffErrors(GCRBackoff(), func() error {
			return c.copyRepo(ctx, repo, tags)
		}); err != nil {
			return fmt.Errorf("failed to copy repo %q: %w", repo, err)
		}

		return nil
	}

	// Start walking the repo, enqueuing items in c.tasks.
	g.Go(func() error {
		defer close(c.tasks)
		if err := google.Walk(c.srcRepo, walkFn, o.google...); err != nil {
			return fmt.Errorf("failed to Walk: %w", err)
		}
		return nil
	})

	// Pull items off of c.tasks and copy the images.
	for i := 0; i < o.jobs; i++ {
		g.Go(func() error {
			for task := range c.tasks {
				// If we hit an error when trying to copy the images,
				// retry with backoff.
				if err := backoffErrors(GCRBackoff(), func() error {
					return c.copyImages(ctx, task)
				}); err != nil {
					return fmt.Errorf("failed to copy %q: %w", task.digest, err)
				}
			}
			return nil
		})
	}

	return g.Wait()
}

// copyRepo figures out the name for our destination repo (newRepo), lists the
// contents of newRepo, calculates the diff of what needs to be copied, then
// starts a goroutine to copy each image we need, and waits for them to finish.
func (c *copier) copyRepo(ctx context.Context, oldRepo name.Repository, tags *google.Tags) error {
	newRepo, err := c.rename(oldRepo)
	if err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}

	// Figure out what we actually need to copy.
	want := tags.Manifests
	have := make(map[string]google.ManifestInfo)
	haveTags, err := google.List(newRepo, c.opt.google...)
	if err != nil {
		if !hasStatusCode(err, http.StatusNotFound) {
			return err
		}
		// This is a 404 code, so we just need to copy everything.
		logs.Warn.Printf("failed to list %s: %v", newRepo, err)
	} else {
		have = haveTags.Manifests
	}
	need := diffImages(want, have)

	// Queue up every image as a task.
	for digest, manifest := range need {
		t := task{
			digest:   digest,
			manifest: manifest,
			oldRepo:  oldRepo,
			newRepo:  newRepo,
		}
		select {
		case c.tasks <- t:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// copyImages starts a goroutine for each tag that points to the image
// oldRepo@digest, or just copies the image by digest if there are no tags.
func (c *copier) copyImages(_ context.Context, t task) error {
	// We only have to explicitly copy by digest if there are no tags pointing to this manifest.
	if len(t.manifest.Tags) == 0 {
		srcImg := fmt.Sprintf("%s@%s", t.oldRepo, t.digest)
		dstImg := fmt.Sprintf("%s@%s", t.newRepo, t.digest)

		return crane.Copy(srcImg, dstImg, c.opt.crane...)
	}

	// We only need to push the whole image once.
	tag := t.manifest.Tags[0]
	srcImg := fmt.Sprintf("%s:%s", t.oldRepo, tag)
	dstImg := fmt.Sprintf("%s:%s", t.newRepo, tag)

	if err := crane.Copy(srcImg, dstImg, c.opt.crane...); err != nil {
		return err
	}

	if len(t.manifest.Tags) <= 1 {
		// If there's only one tag, we're done.
		return nil
	}

	// Add the rest of the tags.
	srcRef, err := name.ParseReference(srcImg)
	if err != nil {
		return err
	}
	desc, err := remote.Get(srcRef, c.opt.remote...)
	if err != nil {
		return err
	}

	for _, tag := range t.manifest.Tags[1:] {
		dstImg := t.newRepo.Tag(tag)

		if err := remote.Tag(dstImg, desc, c.opt.remote...); err != nil {
			return err
		}
	}

	return nil
}

// Retry temporary errors, 429, and 500+ with backoff.
func backoffErrors(bo retry.Backoff, f func() error) error {
	p := func(err error) bool {
		b := retry.IsTemporary(err) || hasStatusCode(err, http.StatusTooManyRequests) || isServerError(err)
		if b {
			logs.Warn.Printf("Retrying %v", err)
		}
		return b
	}
	return retry.Retry(f, p, bo)
}

func hasStatusCode(err error, code int) bool {
	if err == nil {
		return false
	}
	var terr *transport.Error
	if errors.As(err, &terr) {
		if terr.StatusCode == code {
			return true
		}
	}
	return false
}

func isServerError(err error) bool {
	if err == nil {
		return false
	}
	var terr *transport.Error
	if errors.As(err, &terr) {
		return terr.StatusCode >= 500
	}
	return false
}

// rename figures out the name of the new repository to copy to, e.g.:
//
// $ gcrane cp -r gcr.io/foo gcr.io/baz
//
// rename("gcr.io/foo/bar") == "gcr.io/baz/bar"
func (c *copier) rename(repo name.Repository) (name.Repository, error) {
	replaced := strings.Replace(repo.String(), c.srcRepo.String(), c.dstRepo.String(), 1)
	return name.NewRepository(replaced, name.StrictValidation)
}

// diffImages returns a map of digests to google.ManifestInfos for images or
// tags that are present in "want" but not in "have".
func diffImages(want, have map[string]google.ManifestInfo) map[string]google.ManifestInfo {
	need := make(map[string]google.ManifestInfo)

	for digest, wantManifest := range want {
		if haveManifest, ok := have[digest]; !ok {
			// Missing the whole image, we need to copy everything.
			need[digest] = wantManifest
		} else {
			missingTags := subtractStringLists(wantManifest.Tags, haveManifest.Tags)
			if len(missingTags) == 0 {
				continue
			}

			// Missing just some tags, add the ones we need to copy.
			todo := wantManifest
			todo.Tags = missingTags
			need[digest] = todo
		}
	}

	return need
}

// subtractStringLists returns a list of strings that are in minuend and not
// in subtrahend; order is unimportant.
func subtractStringLists(minuend, subtrahend []string) []string {
	bSet := toStringSet(subtrahend)
	difference := []string{}

	for _, a := range minuend {
		if _, ok := bSet[a]; !ok {
			difference = append(difference, a)
		}
	}

	return difference
}

func toStringSet(slice []string) map[string]struct{} {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}
	return set
}
