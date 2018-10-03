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
)

func init() { Root.AddCommand(NewCmdCopy()) }

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

	c, err := newCopier(src, dst)
	if err != nil {
		log.Fatal(err)
	}

	if recursive {
		if err := c.recursiveCopy(src, dst); err != nil {
			log.Fatalf("failed to copy images: %v", err)
		}
	} else {
		if err := c.singleCopy(src, dst); err != nil {
			log.Fatalf("failed to copy image: %v", err)
		}
	}
}

// Workaround for:
// https://github.com/GoogleCloudPlatform/docker-credential-gcr/issues/54
type staticAuth struct {
	auth string
	err  error
}

func (sa staticAuth) Authorization() (string, error) {
	return sa.auth, sa.err
}

type copier struct {
	srcRepo name.Repository
	dstRepo name.Repository

	srcAuth authn.Authenticator
	dstAuth authn.Authenticator

	group *errgroup.Group
	ctx   context.Context
}

func newCopier(src, dst string) (*copier, error) {
	srcRepo, err := name.NewRepository(src, name.WeakValidation)
	if err != nil {
		if srcRef, refErr := name.ParseReference(src, name.WeakValidation); refErr == nil {
			srcRepo = srcRef.Context()
		} else {
			return nil, fmt.Errorf("parsing repo %q: %v", src, err)
		}
	}

	dstRepo, err := name.NewRepository(dst, name.WeakValidation)
	if err != nil {
		if dstRef, refErr := name.ParseReference(dst, name.WeakValidation); refErr == nil {
			dstRepo = dstRef.Context()
		} else {
			return nil, fmt.Errorf("parsing repo %q: %v", dst, err)
		}
	}

	srcHelper, err := authn.DefaultKeychain.Resolve(srcRepo.Registry)
	if err != nil {
		return nil, fmt.Errorf("getting auth for %q: %v", src, err)
	}

	dstHelper, err := authn.DefaultKeychain.Resolve(dstRepo.Registry)
	if err != nil {
		return nil, fmt.Errorf("getting auth for %q: %v", dst, err)
	}

	// TODO(GoogleCloudPlatform/docker-credential-gcr#54): Remove these.
	// This is pretty stupid, but we can't invoke the cred helper as quickly
	// as we need to, so we cache the results. Creds are valid for about an hour,
	// so we'd hit quota issues well before they expired.
	auth, err := srcHelper.Authorization()
	srcAuth := staticAuth{auth, err}

	auth, err = dstHelper.Authorization()
	dstAuth := staticAuth{auth, err}

	// Used so we can collect errors - one goroutine per repo.
	group, ctx := errgroup.WithContext(context.Background())

	return &copier{srcRepo, dstRepo, srcAuth, dstAuth, group, ctx}, nil
}

func (c *copier) singleCopy(src, dst string) error {
	srcRef, err := name.ParseReference(src, name.WeakValidation)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", src, err)
	}

	dstRef, err := name.ParseReference(dst, name.WeakValidation)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", dst, err)
	}

	img, err := remote.Image(srcRef, remote.WithAuth(c.srcAuth))
	if err != nil {
		return fmt.Errorf("reading image %q: %v", srcRef, err)
	}

	if err := remote.Write(dstRef, img, c.dstAuth, http.DefaultTransport); err != nil {
		return fmt.Errorf("writing image %q: %v", dstRef, err)
	}

	return nil
}

func (c *copier) recursiveCopy(src, dst string) error {
	if err := google.Walk(c.srcRepo, c.walkFn, google.WithAuth(c.srcAuth)); err != nil {
		return fmt.Errorf("failed to Walk: %v", err)
	}

	return c.group.Wait()
}

func (c *copier) walkFn(oldRepo name.Repository, tags *google.Tags, err error) error {
	if err != nil {
		return fmt.Errorf("failed to Walk %s: %v", oldRepo, err)
	}

	c.group.Go(func() error {
		return c.copyRepo(oldRepo, tags)
	})

	return nil
}

func (c *copier) rename(repo name.Repository) (name.Repository, error) {
	replaced := strings.Replace(repo.String(), c.srcRepo.String(), c.dstRepo.String(), 1)
	return name.NewRepository(replaced, name.StrictValidation)
}

func (c *copier) copyRepo(oldRepo name.Repository, tags *google.Tags) error {
	newRepo, err := c.rename(oldRepo)
	if err != nil {
		return err
	}

	want := tags.Manifests
	have := make(map[string]google.ManifestInfo)
	haveTags, err := google.List(newRepo, google.WithAuth(c.dstAuth))
	if err != nil {
		// Possibly, we could see a 404.  If we get an error here, assume we need to
		// copy everything. TODO: refactor remote.Error to expose response code?
	} else {
		have = haveTags.Manifests
	}
	need := diffImages(want, have)

	g, ctx := errgroup.WithContext(c.ctx)

	// First go through copying just manifests, skipping manifest lists, since
	// manifest lists might reference them.
	todos := make(map[string]google.ManifestInfo)
	for digest, manifest := range need {
		if manifest.MediaType == string(types.DockerManifestList) || manifest.MediaType == string(types.OCIImageIndex) {
			todos[digest] = manifest
			continue
		}

		digest, manifest := digest, manifest // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			return c.copyImages(ctx, digest, manifest, oldRepo, newRepo)
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("Failed to copy %s: %v", oldRepo, err)
	}

	// TODO: Uncomment once we've implemented manifests lists.
	// Now copy the manifest lists, since it should be safe.
	// for digest, manifest := range todos {
	// 	digest, manifest := digest, manifest // https://golang.org/doc/faq#closures_and_goroutines
	// 	g.Go(func() error {
	// 		return c.copyImages(ctx, digest, manifest, oldRepo, newRepo)
	// 	})
	// }

	// if err := g.Wait(); err != nil {
	// 	return fmt.Errorf("Failed to copy %s: %v", oldRepo, err)
	// }

	return nil
}

func (c *copier) copyImages(ctx context.Context, digest string, manifest google.ManifestInfo, oldRepo, newRepo name.Repository) error {
	// We only have to explicitly copy by digest if there are no tags pointing to this manifest.
	if len(manifest.Tags) == 0 {
		srcImg := fmt.Sprintf("%s@%s", oldRepo, digest)
		dstImg := fmt.Sprintf("%s@%s", newRepo, digest)

		return c.singleCopy(srcImg, dstImg)
	}

	// Copy all the tags.
	g, _ := errgroup.WithContext(ctx)
	for _, tag := range manifest.Tags {
		tag := tag // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			srcImg := fmt.Sprintf("%s:%s", oldRepo, tag)
			dstImg := fmt.Sprintf("%s:%s", newRepo, tag)

			return c.singleCopy(srcImg, dstImg)
		})
	}
	return g.Wait()
}

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
