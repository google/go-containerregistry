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
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
)

// NewCmdRebase creates a new cobra.Command for the rebase subcommand.
func NewCmdRebase(options *[]crane.Option) *cobra.Command {
	var orig, oldBase, newBase, rebased string

	rebaseCmd := &cobra.Command{
		Use:   "rebase",
		Short: "Rebase an image onto a new base image",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if orig == "" {
				orig = args[0]
			} else if len(args) != 0 || args[0] != "" {
				return fmt.Errorf("cannot use --original with positional argument")
			}

			// Parse and fetch the original image or index.
			origRef, err := name.ParseReference(orig)
			if err != nil {
				return fmt.Errorf("parsing tag %q: %v", orig, err)
			}
			origDesc, err := remote.Get(origRef)
			if err != nil {
				return err
			}
			if origDesc.Descriptor.MediaType.IsIndex() {
				return errors.New("rebasing indexes is not currently supported")
			}

			// TODO: This will bias toward rebasing the linux/amd64
			// image, instead of the whole index.  Support rebasing
			// indexes, and support the --platform flag here.
			origImg, err := origDesc.Image()
			if err != nil {
				return err
			}
			origMf, err := origImg.Manifest()
			if err != nil {
				return err
			}
			anns := origMf.Annotations
			if newBase == "" && anns != nil {
				newBase = anns[specsv1.AnnotationBaseImageName]
			}
			if newBase == "" {
				return errors.New("could not determine new base image from annotations")
			}
			newBaseRef, err := name.ParseReference(newBase)
			if err != nil {
				return err
			}
			if oldBase == "" && anns != nil {
				oldBaseDigest := anns[specsv1.AnnotationBaseImageDigest]
				oldBase = newBaseRef.Context().Digest(oldBaseDigest).String()
			}
			if oldBase == "" {
				return errors.New("could not determine old base image by digest from annotations")
			}

			rebasedImg, err := rebaseImage(origImg, oldBase, newBase, *options...)
			if err != nil {
				return fmt.Errorf("rebasing image: %v", err)
			}

			// If the new ref isn't provided, write over the original image.
			// If that ref was provided by digest (e.g., output from
			// another crane command), then strip that and push to
			// a ":rebased" tag instead.
			if rebased == "" {
				if _, ok := origRef.(name.Digest); ok {
					rebased = origRef.Context().Tag("rebased").String()
				} else {
					rebased = orig
				}
			}
			logs.Progress.Println("pushing rebased image as", rebased)
			rebasedDigest, err := rebasedImg.Digest()
			if err != nil {
				return fmt.Errorf("digesting new image: %v", err)
			}
			origDigest, err := origImg.Digest()
			if err != nil {
				return err
			}
			if rebasedDigest == origDigest {
				logs.Warn.Println("rebasing was no-op")
			}

			r, err := name.ParseReference(rebased)
			if err != nil {
				return fmt.Errorf("rebasing: %v", err)
			}

			if err := crane.Push(rebasedImg, rebased, *options...); err != nil {
				return fmt.Errorf("pushing %s: %v", rebased, err)
			}
			if _, ok := r.(name.Digest); ok {
				rebased = r.Context().Digest(rebasedDigest.String()).String()
				if err := crane.Push(rebasedImg, rebased, *options...); err != nil {
					return fmt.Errorf("pushing %s: %v", rebased, err)
				}
			}

			rebasedRef, err := name.ParseReference(rebased)
			if err != nil {
				return fmt.Errorf("parsing %q: %v", rebased, err)
			}

			fmt.Println(rebasedRef.Context().Digest(rebasedDigest.String()))
			return nil
		},
	}
	rebaseCmd.Flags().StringVar(&orig, "original", "", "Original image to rebase; use positional arg instead")
	rebaseCmd.Flags().StringVar(&oldBase, "old_base", "", "Old base image to remove")
	rebaseCmd.Flags().StringVar(&newBase, "new_base", "", "New base image to insert")
	rebaseCmd.Flags().StringVarP(&rebased, "tag", "t", "", "Tag to apply to rebased image")
	return rebaseCmd
}

// rebaseImage parses the references and uses them to perform a rebase on the
// original image.
//
// If oldBase or newBase are "", rebaseImage attempts to derive them using
// annotations in the original image. If those annotations are not found,
// rebaseImage returns an error.
//
// If rebasing is successful, base image annotations are set on the resulting
// image to facilitate implicit rebasing next time.
func rebaseImage(orig v1.Image, oldBase, newBase string, opt ...crane.Option) (v1.Image, error) {
	m, err := orig.Manifest()
	if err != nil {
		return nil, err
	}
	if newBase == "" && m.Annotations != nil {
		newBase = m.Annotations[specsv1.AnnotationBaseImageName]
		if newBase != "" {
			logs.Debug.Printf("Detected new base from %q annotation: %s", specsv1.AnnotationBaseImageName, newBase)
		}
	}
	if newBase == "" {
		return nil, fmt.Errorf("either new base or %q annotation is required", specsv1.AnnotationBaseImageName)
	}
	newBaseImg, err := crane.Pull(newBase, opt...)
	if err != nil {
		return nil, err
	}

	if oldBase == "" && m.Annotations != nil {
		oldBase = m.Annotations[specsv1.AnnotationBaseImageDigest]
		if oldBase != "" {
			newBaseRef, err := name.ParseReference(newBase)
			if err != nil {
				return nil, err
			}

			oldBase = newBaseRef.Context().Digest(oldBase).String()
			logs.Debug.Printf("Detected old base from %q annotation: %s", specsv1.AnnotationBaseImageDigest, oldBase)
		}
	}
	if oldBase == "" {
		return nil, fmt.Errorf("either old base or %q annotation is required", specsv1.AnnotationBaseImageDigest)
	}

	oldBaseImg, err := crane.Pull(oldBase, opt...)
	if err != nil {
		return nil, err
	}

	// NB: if newBase is an index, we need to grab the index's digest to
	// annotate the resulting image, even though we pull the
	// platform-specific image to rebase.
	newBaseDigest, err := crane.Digest(newBase, opt...)
	if err != nil {
		return nil, err
	}

	rebased, err := mutate.Rebase(orig, oldBaseImg, newBaseImg)
	if err != nil {
		return nil, err
	}

	// Update base image annotations for the new image manifest.
	logs.Debug.Printf("Setting annotation %q: %q", specsv1.AnnotationBaseImageDigest, newBaseDigest)
	logs.Debug.Printf("Setting annotation %q: %q", specsv1.AnnotationBaseImageName, newBase)
	return mutate.Annotations(rebased, map[string]string{
		specsv1.AnnotationBaseImageDigest: newBaseDigest,
		specsv1.AnnotationBaseImageName:   newBase,
	}).(v1.Image), nil
}
