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
	"fmt"
	"io/ioutil"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
)

// NewCmdPush creates a new cobra.Command for the push subcommand.
func NewCmdPush(options *[]crane.Option) *cobra.Command {
	index := false
	imageRefs := ""
	cmd := &cobra.Command{
		Use:   "push PATH IMAGE",
		Short: "Push local image contents to a remote registry",
		Long:  `If the PATH is a directory, it will be read as an OCI image layout. Otherwise, PATH is assumed to be a docker-style tarball.`,
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
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

			// If the destination contains a digest then we should only push the manifest with that digest
			if digestRef, ok := ref.(name.Digest); ok {
				// convert the string to a v1.Hash
				hashRef, err := v1.NewHash(digestRef.DigestStr())
				if err != nil {
					return err
				}

				switch t := img.(type) {
				case v1.Image:
					// ensure that the digest matches
					hh, err := t.Digest()
					if err != nil {
						return err
					}
					if hashRef != hh {
						return fmt.Errorf("requested manifest not present in index, missing %s", hh)
					}
				case v1.ImageIndex:
					// Get a specific image
					// Try ImageIndex first
					img, err = t.ImageIndex(hashRef)
					if err == nil {
						break
					}
					// Fallback to plain Image
					img, err = t.Image(hashRef)
					if err != nil {
						return err
					}
					// TODO there is no good way to handle the errors here since they are untyped and they contain information we do not know (the media type of the match).
					//  I wish the findDescriptor() function was exposed or there was a more tolerant call to "Image" that did not care about Manifest vs Manifest List.
				}
			}

			var h v1.Hash
			switch t := img.(type) {
			case v1.Image:
				if err := remote.Write(ref, t, o.Remote...); err != nil {
					return err
				}
				if h, err = t.Digest(); err != nil {
					return err
				}
			case v1.ImageIndex:
				if err := remote.WriteIndex(ref, t, o.Remote...); err != nil {
					return err
				}
				if h, err = t.Digest(); err != nil {
					return err
				}
			default:
				return fmt.Errorf("cannot push type (%T) to registry", img)
			}

			digest := ref.Context().Digest(h.String())
			if imageRefs != "" {
				return ioutil.WriteFile(imageRefs, []byte(digest.String()), 0600)
			}
			// TODO(mattmoor): think about printing the digest to standard out
			// to facilitate command composition similar to ko build.

			return nil
		},
	}
	cmd.Flags().BoolVar(&index, "index", false, "push a collection of images as a single index, currently required if PATH contains multiple images")
	cmd.Flags().StringVar(&imageRefs, "image-refs", "", "path to file where a list of the published image references will be written")
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
