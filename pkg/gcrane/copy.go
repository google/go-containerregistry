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
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

func init() { Root.AddCommand(NewCmdCopy()) }

// NewCmdCopy creates a new cobra.Command for the copy subcommand.
func NewCmdCopy() *cobra.Command {
	recursive := false
	cmd := &cobra.Command{
		Use:     "copy",
		Aliases: []string{"cp"},
		Short:   "Efficiently copy a remote image from src to dst",
		Args:    cobra.ExactArgs(2),
		Run: func(cc *cobra.Command, args []string) {
			doCopy(args, recursive)
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Whether to recurse through repos")

	return cmd
}

func doCopy(args []string, recursive bool) {
	src, dst := args[0], args[1]

	if recursive {
		if err := recursiveCopy(src, dst); err != nil {
			log.Fatalf("failed to copy images: %v", err)
		}
	} else {
		srcAuth, dstAuth, err := parseRefAuths(src, dst)
		if err != nil {
			log.Fatal(err)
		}

		// First, try to copy as an index.
		// If that fails, try to copy as an image.
		// We have to try this second because fallback logic exists in the registry
		// to convert an index to an image.
		//
		// TODO(#407): Refactor crane so we can just call into that logic in the
		// single-image case.
		if err := copyIndex(src, dst, srcAuth, dstAuth, http.DefaultTransport); err != nil {
			if err := copyImage(src, dst, srcAuth, dstAuth, http.DefaultTransport); err != nil {
				log.Fatalf("failed to copy image: %v", err)
			}
		}
	}
}

type copier struct {
	srcRepo name.Repository
	dstRepo name.Repository

	srcAuth authn.Authenticator
	dstAuth authn.Authenticator

	transport http.RoundTripper
}

func newCopier(src, dst string, t http.RoundTripper) (*copier, error) {
	srcRepo, err := name.NewRepository(src)
	if err != nil {
		return nil, fmt.Errorf("parsing repo %q: %v", src, err)
	}

	dstRepo, err := name.NewRepository(dst)
	if err != nil {
		return nil, fmt.Errorf("parsing repo %q: %v", dst, err)
	}

	srcAuth, err := google.Keychain.Resolve(srcRepo.Registry)
	if err != nil {
		return nil, fmt.Errorf("getting auth for %q: %v", src, err)
	}

	dstAuth, err := google.Keychain.Resolve(dstRepo.Registry)
	if err != nil {
		return nil, fmt.Errorf("getting auth for %q: %v", dst, err)
	}

	return &copier{srcRepo, dstRepo, srcAuth, dstAuth, t}, nil
}

func copyImage(src, dst string, srcAuth, dstAuth authn.Authenticator, t http.RoundTripper) error {
	srcRef, err := name.ParseReference(src)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", src, err)
	}

	dstRef, err := name.ParseReference(dst)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", dst, err)
	}

	img, err := remote.Image(srcRef, remote.WithAuth(srcAuth), remote.WithTransport(t))
	if err != nil {
		return fmt.Errorf("reading image %q: %v", src, err)
	}

	if err := remote.Write(dstRef, img, dstAuth, t); err != nil {
		return fmt.Errorf("writing image %q: %v", dst, err)
	}

	return nil
}

func copyIndex(src, dst string, srcAuth, dstAuth authn.Authenticator, t http.RoundTripper) error {
	srcRef, err := name.ParseReference(src)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", src, err)
	}

	dstRef, err := name.ParseReference(dst)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", dst, err)
	}

	idx, err := remote.Index(srcRef, remote.WithAuth(srcAuth), remote.WithTransport(t))
	if err != nil {
		return fmt.Errorf("reading image %q: %v", src, err)
	}

	if err := remote.WriteIndex(dstRef, idx, dstAuth, t); err != nil {
		return fmt.Errorf("writing image %q: %v", dst, err)
	}

	return nil
}

type throttledTransport struct {
	transport http.RoundTripper
	limiter   *rate.Limiter
}

func newTransport(t http.RoundTripper, qps, burst int) *throttledTransport {
	return &throttledTransport{
		transport: t,
		limiter:   rate.NewLimiter(rate.Limit(qps), burst),
	}
}

func (t *throttledTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Block until we're allowed to keep going.
	if err := t.limiter.Wait(context.Background()); err != nil {
		return nil, err
	}
	return t.transport.RoundTrip(req)
}

type copyTask struct {
	digest   string
	manifest google.ManifestInfo
	src      name.Repository
	dst      name.Repository
}

// recursiveCopy copies images from repo src to repo dst, rather quickly.
func recursiveCopy(src, dst string) error {
	// We need to throttle our transport so we don't get 429s.
	// QPS based on https://cloud.google.com/container-registry/quotas
	qps := 30000 / (10 * 60)
	burst := 100
	t := newTransport(http.DefaultTransport, qps, burst)

	c, err := newCopier(src, dst, t)
	if err != nil {
		return err
	}

	// Allow some buffer so we can keep walking repos while copying images.
	tasks := make(chan copyTask, 100)

	// Captures c and tasks.
	walkFn := func(repo name.Repository, tags *google.Tags, err error) error {
		if err != nil {
			return fmt.Errorf("failed walkFn for repo %s: %v", repo, err)
		}

		todos, err := c.tasksForRepo(repo, tags)
		if err != nil {
			return err
		}

		// Blocks if we have enqueued too many images. This ensures we make forward
		// progress on the image copies instead of consuming all of our QPS listing
		// images.
		for _, task := range todos {
			tasks <- task
		}

		return nil
	}

	g, _ := errgroup.WithContext(context.Background())

	// Start walking the source, finding images to copy. For registries with a
	// large number of repositories, this could be faster, but we will probably
	// be more limited by our copying throughput anyway.
	g.Go(func() error {
		defer close(tasks)
		return google.Walk(c.srcRepo, walkFn, google.WithAuth(c.srcAuth), google.WithTransport(c.transport))
	})

	// Start a fixed number of goroutines to copy. Inspired by errgroup example.
	const copiers = 20
	for i := 0; i < copiers; i++ {
		g.Go(func() error {
			for task := range tasks {
				switch types.MediaType(task.manifest.MediaType) {
				case types.DockerManifestList, types.OCIImageIndex:
					if err := c.copyImages(task.digest, task.manifest, task.src, task.dst, copyIndex); err != nil {
						return fmt.Errorf("failed to copy %s@%s: %v", task.src, task.digest, err)
					}
				case types.DockerManifestSchema2, types.OCIManifestSchema1:
					if err := c.copyImages(task.digest, task.manifest, task.src, task.dst, copyImage); err != nil {
						return fmt.Errorf("failed to copy %s@%s: %v", task.src, task.digest, err)
					}
				default:
					// Just log this and don't consider it fatal.
					fmt.Printf("skipping %s@%s: unexpected mediaType: %s\n", task.src, task.digest, task.manifest.MediaType)
				}
			}
			return nil
		})
	}

	return g.Wait()
}

// tasksForRepo figures out the name for our destination repo (newRepo), lists the
// contents of newRepo, calculates the diff of what needs to be copied, and returns
// a task for each image that needs to be copied.
func (c *copier) tasksForRepo(oldRepo name.Repository, tags *google.Tags) ([]copyTask, error) {
	newRepo, err := c.rename(oldRepo)
	if err != nil {
		return nil, fmt.Errorf("rename failed: %v", err)
	}

	// Figure out what we actually need to copy.
	want := tags.Manifests
	have := make(map[string]google.ManifestInfo)
	haveTags, err := google.List(newRepo, google.WithAuth(c.dstAuth), google.WithTransport(c.transport))
	if err != nil {
		// Possibly, we could see a 404.  If we get an error here, log it and assume
		// we just need to copy everything.
		//
		// TODO: refactor remote.Error to expose response code?
		log.Printf("failed to list %s: %v", newRepo, err)
	} else {
		have = haveTags.Manifests
	}

	tasks := []copyTask{}
	for digest, manifest := range diffImages(want, have) {
		tasks = append(tasks, copyTask{
			digest:   digest,
			manifest: manifest,
			src:      oldRepo,
			dst:      newRepo,
		})
	}

	return tasks, nil
}

// copyFunc is the signature shared by copyImage and copyIndex
type copyFunc func(src, dst string, srcAuth, dstAuth authn.Authenticator, t http.RoundTripper) error

// copyImages copies each tag that points to the image oldRepo@digest, or just
// copies the image by digest if there are no tags.
func (c *copier) copyImages(digest string, manifest google.ManifestInfo, oldRepo, newRepo name.Repository, copyFn copyFunc) error {
	// We only have to explicitly copy by digest if there are no tags pointing to this manifest.
	if len(manifest.Tags) == 0 {
		srcImg := fmt.Sprintf("%s@%s", oldRepo, digest)
		dstImg := fmt.Sprintf("%s@%s", newRepo, digest)

		return copyFn(srcImg, dstImg, c.srcAuth, c.dstAuth, c.transport)
	}

	// Copy all the tags.
	for _, tag := range manifest.Tags {
		srcImg := fmt.Sprintf("%s:%s", oldRepo, tag)
		dstImg := fmt.Sprintf("%s:%s", newRepo, tag)

		if err := copyFn(srcImg, dstImg, c.srcAuth, c.dstAuth, c.transport); err != nil {
			return err
		}
	}

	return nil
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

func parseRefAuths(src, dst string) (authn.Authenticator, authn.Authenticator, error) {
	srcRef, err := name.ParseReference(src)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing reference %q: %v", src, err)
	}

	dstRef, err := name.ParseReference(dst)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing reference %q: %v", dst, err)
	}

	srcAuth, err := authn.DefaultKeychain.Resolve(srcRef.Context().Registry)
	if err != nil {
		return nil, nil, fmt.Errorf("getting auth for %q: %v", src, err)
	}

	dstAuth, err := authn.DefaultKeychain.Resolve(dstRef.Context().Registry)
	if err != nil {
		return nil, nil, fmt.Errorf("getting auth for %q: %v", dst, err)
	}

	return srcAuth, dstAuth, nil
}
