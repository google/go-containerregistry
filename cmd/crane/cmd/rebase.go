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
	"maps"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
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
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if orig == "" {
				orig = args[0]
			} else if len(args) != 0 || args[0] != "" {
				return fmt.Errorf("cannot use --original with positional argument")
			}

			// If the new ref isn't provided, write over the original image.
			// If that ref was provided by digest (e.g., output from
			// another crane command), then strip that and push the
			// mutated image by digest instead.
			if rebased == "" {
				rebased = orig
			}

			o := crane.GetOptions(*options...)
			r, err := name.ParseReference(rebased, o.Name...)
			if err != nil {
				return fmt.Errorf("parsing %s: %w", rebased, err)
			}

			desc, err := crane.Get(orig, *options...)
			if err != nil {
				return fmt.Errorf("checking %s: %w", orig, err)
			}

			// Without --platform, an index is rebased child by child.
			if o.Platform == nil && desc.MediaType.IsIndex() {
				origIdx, err := desc.ImageIndex()
				if err != nil {
					return err
				}
				rebasedIdx, err := rebaseIndex(origIdx, oldBase, newBase, *options...)
				if err != nil {
					return fmt.Errorf("rebasing index: %w", err)
				}
				rebasedDigest, err := rebasedIdx.Digest()
				if err != nil {
					return fmt.Errorf("digesting new index: %w", err)
				}
				origDigest, err := origIdx.Digest()
				if err != nil {
					return err
				}
				if rebasedDigest == origDigest {
					logs.Warn.Println("rebasing was no-op")
				}

				if _, ok := r.(name.Digest); ok {
					r = r.Context().Digest(rebasedDigest.String())
				}
				logs.Progress.Println("pushing rebased index as", r)
				if err := remote.WriteIndex(r, rebasedIdx, o.Remote...); err != nil {
					return fmt.Errorf("pushing %s: %w", r, err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), r.Context().Digest(rebasedDigest.String()))
				return nil
			}

			origImg, err := desc.Image()
			if err != nil {
				return err
			}
			rebasedImg, err := rebaseImage(origImg, oldBase, newBase, *options...)
			if err != nil {
				return fmt.Errorf("rebasing image: %w", err)
			}

			rebasedDigest, err := rebasedImg.Digest()
			if err != nil {
				return fmt.Errorf("digesting new image: %w", err)
			}
			origDigest, err := origImg.Digest()
			if err != nil {
				return err
			}
			if rebasedDigest == origDigest {
				logs.Warn.Println("rebasing was no-op")
			}

			if _, ok := r.(name.Digest); ok {
				r = r.Context().Digest(rebasedDigest.String())
			}
			logs.Progress.Println("pushing rebased image as", r)
			if err := crane.Push(rebasedImg, r.String(), *options...); err != nil {
				return fmt.Errorf("pushing %s: %w", r, err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), r.Context().Digest(rebasedDigest.String()))
			return nil
		},
	}
	rebaseCmd.Flags().StringVar(&orig, "original", "", "Original image to rebase (DEPRECATED: use positional arg instead)")
	rebaseCmd.Flags().StringVar(&oldBase, "old_base", "", "Old base image to remove")
	rebaseCmd.Flags().StringVar(&newBase, "new_base", "", "New base image to insert")
	rebaseCmd.Flags().StringVar(&rebased, "rebased", "", "Tag to apply to rebased image (DEPRECATED: use --tag)")
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
	// crane.Digest will pull a platform-specific image, so use crane.Head
	// here instead.
	newBaseDesc, err := crane.Head(newBase, opt...)
	if err != nil {
		return nil, err
	}
	newBaseDigest := newBaseDesc.Digest.String()

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

// rebaseIndex rebases each child manifest of orig onto newBase and returns an
// index with orig's index-level fields, the rebased children, and base image
// annotations for newBase. Each child is rebased onto the base image for its
// own platform, so newBase and oldBase should be multi-platform indexes.
//
// If oldBase or newBase are "", rebaseIndex derives them from orig's
// annotations, as rebaseImage does.
//
// Attestation manifests (platform unknown/unknown) are dropped, as flattenIndex
// does: they describe the pre-rebase image digests.
func rebaseIndex(orig v1.ImageIndex, oldBase, newBase string, opt ...crane.Option) (v1.ImageIndex, error) {
	m, err := orig.IndexManifest()
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
	if oldBase == "" && m.Annotations != nil {
		if oldBaseDigest := m.Annotations[specsv1.AnnotationBaseImageDigest]; oldBaseDigest != "" {
			newBaseRef, err := name.ParseReference(newBase)
			if err != nil {
				return nil, err
			}
			oldBase = newBaseRef.Context().Digest(oldBaseDigest).String()
			logs.Debug.Printf("Detected old base from %q annotation: %s", specsv1.AnnotationBaseImageDigest, oldBase)
		}
	}
	if oldBase == "" {
		return nil, fmt.Errorf("either old base or %q annotation is required", specsv1.AnnotationBaseImageDigest)
	}

	// Record the index digest of newBase, as rebaseImage does for images.
	newBaseDesc, err := crane.Head(newBase, opt...)
	if err != nil {
		return nil, err
	}
	newBaseDigest := newBaseDesc.Digest.String()

	adds := make([]mutate.IndexAddendum, 0, len(m.Manifests))
	for _, desc := range m.Manifests {
		// Keep the parent entry's descriptor, not the descriptor resolved by
		// digest: multiple entries may share content but have different metadata.
		if p := desc.Platform; p != nil && p.OS == "unknown" && p.Architecture == "unknown" {
			continue
		}
		// Keep existing base annotations in step with the rebased content,
		// whether the entry points to an image or a nested index.
		if desc.Annotations != nil {
			desc.Annotations = maps.Clone(desc.Annotations)
			if _, ok := desc.Annotations[specsv1.AnnotationBaseImageName]; ok {
				desc.Annotations[specsv1.AnnotationBaseImageName] = newBase
			}
			if _, ok := desc.Annotations[specsv1.AnnotationBaseImageDigest]; ok {
				desc.Annotations[specsv1.AnnotationBaseImageDigest] = newBaseDigest
			}
		}

		switch {
		case desc.MediaType.IsImage():
			c, err := orig.Image(desc.Digest)
			if err != nil {
				return nil, err
			}
			// Use the base image for the child's platform. The config supplies
			// the platform when the descriptor has none.
			plat := desc.Platform
			if plat == nil {
				cf, err := c.ConfigFile()
				if err != nil {
					return nil, err
				}
				plat = cf.Platform()
			}
			childOpts := append([]crane.Option{}, opt...)
			if plat != nil {
				childOpts = append(childOpts, crane.WithPlatform(plat))
			}
			rebasedChild, err := rebaseImage(c, oldBase, newBase, childOpts...)
			if err != nil {
				return nil, err
			}
			if err := setDescriptorContent(&desc, rebasedChild); err != nil {
				return nil, err
			}
			adds = append(adds, mutate.IndexAddendum{Add: rebasedChild, Descriptor: desc})
		case desc.MediaType.IsIndex():
			c, err := orig.ImageIndex(desc.Digest)
			if err != nil {
				return nil, err
			}
			rebasedChild, err := rebaseIndex(c, oldBase, newBase, opt...)
			if err != nil {
				return nil, err
			}
			if err := setDescriptorContent(&desc, rebasedChild); err != nil {
				return nil, err
			}
			adds = append(adds, mutate.IndexAddendum{Add: rebasedChild, Descriptor: desc})
		default:
			return nil, fmt.Errorf("unexpected child manifest media type %q", desc.MediaType)
		}
	}

	// Rebuild from orig so its media type, artifact type, subject, and
	// existing annotations are kept; only the children change.
	idx := mutate.RemoveManifests(orig, func(v1.Descriptor) bool { return true })
	idx = mutate.AppendManifests(idx, adds...)
	return mutate.Annotations(idx, map[string]string{
		specsv1.AnnotationBaseImageDigest: newBaseDigest,
		specsv1.AnnotationBaseImageName:   newBase,
	}).(v1.ImageIndex), nil
}

// setDescriptorContent points desc at the rebased manifest m. It updates the
// size, digest, and media type, and replaces inline data (when desc has any)
// with m's manifest so the data still matches the digest. Other fields of desc
// are kept.
func setDescriptorContent(desc *v1.Descriptor, m interface {
	partial.Describable
	partial.WithRawManifest
}) error {
	rd, err := partial.Descriptor(m)
	if err != nil {
		return err
	}
	desc.Size, desc.Digest, desc.MediaType = rd.Size, rd.Digest, rd.MediaType
	if desc.Data != nil {
		raw, err := m.RawManifest()
		if err != nil {
			return err
		}
		desc.Data = raw
	}
	return nil
}
