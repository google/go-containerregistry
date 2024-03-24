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

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type imageWithRef struct {
	image partial.WithRawManifest
	ref   name.Reference
}

// NewCmdPush creates a new cobra.Command for the push subcommand.
func NewCmdPush(options *[]crane.Option) *cobra.Command {
	var (
		imageRefs          string
		index, annotateRef bool
	)
	cmd := &cobra.Command{
		Use:   "push PATH IMAGE",
		Short: "Push local image contents to a remote registry",
		Long:  `If the PATH is a directory, it will be read as an OCI image layout. Otherwise, PATH is assumed to be a docker-style tarball.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !annotateRef {
				if err := cobra.ExactArgs(2)(cmd, args); err != nil {
					return err
				}
				path, tag := args[0], args[1]
				img, err := loadImage(path, index)
				if err != nil {
					return err
				}

				o := crane.GetOptions(*options...)
				ref, err := name.ParseReference(tag, o.Name...)
				if err != nil {
					return err
				}

				digest, err := writeImage(ref, img, o.Remote...)
				if err != nil {
					return err
				}
				if imageRefs != "" {
					if err := os.WriteFile(imageRefs, []byte(digest.String()), 0600); err != nil {
						return fmt.Errorf("failed to write image refs to %s: %w", imageRefs, err)
					}
				}

				// Print the digest of the pushed image to stdout to facilitate command composition.
				fmt.Fprintln(cmd.OutOrStdout(), digest)
			} else {
				if err := cobra.RangeArgs(1, 2)(cmd, args); err != nil {
					return err
				}
				path := args[0]
				var registry *name.Registry
				if len(args) == 2 {
					r, err := name.NewRegistry(args[1])
					if err != nil {
						return err
					}
					registry = &r
				}
				imgRefs, err := loadImageWithRef(path, index)
				if err != nil {
					return err
				}
				pusher, err := remote.NewPusher()
				if err != nil {
					return err
				}
				ctx, cancel := context.WithCancel(cmd.Context())
				defer cancel()
				o := crane.GetOptions(*options...)
				o.Remote = append(o.Remote, remote.WithContext(ctx), remote.Reuse[*remote.Pusher](pusher))
				wg := sync.WaitGroup{}
				var digests []string
				for i := range imgRefs {
					wg.Add(1)
					go func(imgRef imageWithRef) (err error) {
						defer func() {
							if err != nil {
								fmt.Println(err)
								cancel()
							}
							wg.Done()
						}()
						if registry != nil {
							switch t := imgRef.ref.(type) {
							case name.Tag:
								t.Registry = *registry
								imgRef.ref = t
							case name.Digest:
								t.Registry = *registry
								imgRef.ref = t
							}
						}
						digest, err := writeImage(imgRef.ref, imgRef.image, o.Remote...)
						if err != nil {
							return err
						}

						if imageRefs != "" {
							digests = append(digests, digest.String())
						}

						// Print the digest of the pushed image to stdout to facilitate command composition.
						fmt.Fprintln(cmd.OutOrStdout(), digest)
						return nil
					}(imgRefs[i])
				}
				wg.Wait()
				if imageRefs != "" {
					return os.WriteFile(imageRefs, []byte(strings.Join(digests, "\n")), 0600)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&index, "index", false, "push a collection of images as a single index, currently required if PATH contains multiple images")
	cmd.Flags().StringVar(&imageRefs, "image-refs", "", "path to file where a list of the published image references will be written")
	cmd.Flags().BoolVar(&annotateRef, "annotate-ref", false, "use image reference to push bundle")
	return cmd
}

func loadImage(path string, index bool) (partial.WithRawManifest, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		img, err := crane.Load(path)
		if err != nil {
			return nil, fmt.Errorf("loading %s as tarball: %w", path, err)
		}
		return img, nil
	}

	l, err := layout.ImageIndexFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("loading %s as OCI layout: %w", path, err)
	}

	if index {
		return l, nil
	}

	m, err := l.IndexManifest()
	if err != nil {
		return nil, err
	}
	if len(m.Manifests) != 1 {
		return nil, fmt.Errorf("layout contains %d entries, consider --index", len(m.Manifests))
	}

	desc := m.Manifests[0]
	if desc.MediaType.IsImage() {
		return l.Image(desc.Digest)
	} else if desc.MediaType.IsIndex() {
		return l.ImageIndex(desc.Digest)
	}

	return nil, fmt.Errorf("layout contains non-image (mediaType: %q), consider --index", desc.MediaType)
}

func loadImageWithRef(path string, index bool) ([]imageWithRef, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		imgs, err := tarball.ImageAllFromPath(path)
		if err != nil {
			return nil, fmt.Errorf("loading %s as tarball: %w", path, err)
		}
		var imgRefs []imageWithRef
		for _, img := range imgs {
			imgTagged, ok := img.(partial.WithRepoTags)
			if !ok || len(imgTagged.RepoTags()) == 0 {
				return nil, fmt.Errorf("image %s has no tag", path)
			}
			for _, repoTag := range imgTagged.RepoTags() {
				ref, err := name.ParseReference(repoTag, name.StrictValidation)
				if err != nil {
					return nil, fmt.Errorf("parsing %s repoTag: %w", path, err)
				}
				imgRefs = append(imgRefs, imageWithRef{img, ref})
			}
		}
		return imgRefs, nil
	}

	l, err := layout.ImageIndexFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("loading %s as OCI layout: %w", path, err)
	}

	m, err := l.IndexManifest()
	if err != nil {
		return nil, err
	}

	if index {
		refName := m.Annotations[imagespec.AnnotationRefName]
		ref, err := name.ParseReference(refName, name.StrictValidation)
		if err != nil {
			return nil, fmt.Errorf("parsing %s repoTag: %w", path, err)
		}
		return []imageWithRef{{l, ref}}, err
	}

	imgRefs := make([]imageWithRef, len(m.Manifests))
	for i := range m.Manifests {
		refName := m.Manifests[i].Annotations[imagespec.AnnotationRefName]
		ref, err := name.ParseReference(refName, name.StrictValidation)
		if err != nil {
			return nil, fmt.Errorf("parsing %s repoTag: %w", path, err)
		}
		if m.Manifests[i].MediaType.IsImage() {
			img, err := l.Image(m.Manifests[i].Digest)
			if err != nil {
				return nil, err
			}
			imgRefs[i] = imageWithRef{img, ref}
		} else if m.Manifests[i].MediaType.IsIndex() {
			img, err := l.ImageIndex(m.Manifests[i].Digest)
			if err != nil {
				return nil, err
			}
			imgRefs[i] = imageWithRef{img, ref}
		} else {
			return nil, fmt.Errorf("layout contains unexpected mediaType: %q", m.Manifests[i].MediaType)
		}
	}
	return imgRefs, nil
}

func writeImage(ref name.Reference, img partial.WithRawManifest, options ...remote.Option) (digest name.Digest, err error) {
	var h v1.Hash
	switch t := img.(type) {
	case v1.Image:
		if err = remote.Write(ref, t, options...); err != nil {
			return
		}
		if h, err = t.Digest(); err != nil {
			return
		}
	case v1.ImageIndex:
		if err = remote.WriteIndex(ref, t, options...); err != nil {
			return
		}
		if h, err = t.Digest(); err != nil {
			return
		}
	default:
		return digest, fmt.Errorf("cannot push type (%T) to registry", img)
	}
	return ref.Context().Digest(h.String()), nil
}
