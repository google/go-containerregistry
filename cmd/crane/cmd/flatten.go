// Copyright 2021 Google LLC All Rights Reserved.
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
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/spf13/cobra"
)

// NewCmdFlatten creates a new cobra.Command for the flatten subcommand.
func NewCmdFlatten(options *[]crane.Option) *cobra.Command {
	var dst string
	var lastN int

	flattenCmd := &cobra.Command{
		Use:   "flatten",
		Short: "Flatten an image's layers into a single layer",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// We need direct access to the underlying remote options because crane
			// doesn't expose great facilities for working with an index (yet).
			o := crane.GetOptions(*options...)

			// Pull image and get config.
			src := args[0]

			// If the new ref isn't provided, write over the original image.
			// If that ref was provided by digest (e.g., output from
			// another crane command), then strip that and push the
			// mutated image by digest instead.
			if dst == "" {
				dst = src
			}

			ref, err := name.ParseReference(src, o.Name...)
			if err != nil {
				log.Fatalf("parsing %s: %v", src, err)
			}
			newRef, err := name.ParseReference(dst, o.Name...)
			if err != nil {
				log.Fatalf("parsing %s: %v", dst, err)
			}
			repo := newRef.Context()

			flat, err := flatten(ref, repo, cmd.Parent().Use, o, lastN)
			if err != nil {
				log.Fatalf("flattening %s: %v", ref, err)
			}

			digest, err := flat.Digest()
			if err != nil {
				log.Fatalf("digesting new image: %v", err)
			}

			if _, ok := ref.(name.Digest); ok {
				newRef = repo.Digest(digest.String())
			}

			if err := push(flat, newRef, o); err != nil {
				log.Fatalf("pushing %s: %v", newRef, err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), repo.Digest(digest.String()))
		},
	}
	flattenCmd.Flags().StringVarP(&dst, "tag", "t", "", "New tag to apply to flattened image. If not provided, push by digest to the original image repository.")
	flattenCmd.Flags().IntVarP(&lastN, "last-n-layers", "n", 0, "Only flatten the last N layers (0 = flatten all layers).")
	flattenCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if lastN < 0 {
			return fmt.Errorf("--last-n-layers (-n) must be >= 0")
		}
		return nil
	}
	return flattenCmd
}

func flatten(ref name.Reference, repo name.Repository, use string, o crane.Options, lastN int) (partial.Describable, error) {
	desc, err := remote.Get(ref, o.Remote...)
	if err != nil {
		return nil, fmt.Errorf("pulling %s: %w", ref, err)
	}

	if desc.MediaType.IsIndex() {
		idx, err := desc.ImageIndex()
		if err != nil {
			return nil, err
		}
		return flattenIndex(idx, repo, use, o, lastN)
	} else if desc.MediaType.IsImage() {
		img, err := desc.Image()
		if err != nil {
			return nil, err
		}
		return flattenImage(img, repo, use, o, lastN)
	}

	return nil, fmt.Errorf("can't flatten %s", desc.MediaType)
}

func push(flat partial.Describable, ref name.Reference, o crane.Options) error {
	if idx, ok := flat.(v1.ImageIndex); ok {
		return remote.WriteIndex(ref, idx, o.Remote...)
	} else if img, ok := flat.(v1.Image); ok {
		return remote.Write(ref, img, o.Remote...)
	}

	return fmt.Errorf("can't push %T", flat)
}

func flattenIndex(old v1.ImageIndex, repo name.Repository, use string, o crane.Options, lastN int) (partial.Describable, error) {
	m, err := old.IndexManifest()
	if err != nil {
		return nil, err
	}

	manifests, err := partial.Manifests(old)
	if err != nil {
		return nil, err
	}

	adds := []mutate.IndexAddendum{}

	for _, m := range manifests {
		// Keep the old descriptor (annotations and whatnot).
		desc, err := partial.Descriptor(m)
		if err != nil {
			return nil, err
		}

		// Drop attestations (for now).
		// https://github.com/google/go-containerregistry/issues/1622
		if p := desc.Platform; p != nil {
			if p.OS == "unknown" && p.Architecture == "unknown" {
				continue
			}
		}

		flattened, err := flattenChild(m, repo, use, o, lastN)
		if err != nil {
			return nil, err
		}
		desc.Size, err = flattened.Size()
		if err != nil {
			return nil, err
		}
		desc.Digest, err = flattened.Digest()
		if err != nil {
			return nil, err
		}
		adds = append(adds, mutate.IndexAddendum{
			Add:        flattened,
			Descriptor: *desc,
		})
	}

	idx := mutate.AppendManifests(empty.Index, adds...)

	// Retain any annotations from the original index.
	if len(m.Annotations) != 0 {
		idx = mutate.Annotations(idx, m.Annotations).(v1.ImageIndex)
	}

	// This is stupid, but some registries get mad if you try to push OCI media types that reference docker media types.
	mt, err := old.MediaType()
	if err != nil {
		return nil, err
	}
	idx = mutate.IndexMediaType(idx, mt)

	return idx, nil
}

func flattenChild(old partial.Describable, repo name.Repository, use string, o crane.Options, lastN int) (partial.Describable, error) {
	if idx, ok := old.(v1.ImageIndex); ok {
		return flattenIndex(idx, repo, use, o, lastN)
	} else if img, ok := old.(v1.Image); ok {
		return flattenImage(img, repo, use, o, lastN)
	}

	logs.Warn.Printf("can't flatten %T, skipping", old)
	return old, nil
}

func flattenImage(old v1.Image, repo name.Repository, use string, o crane.Options, lastN int) (partial.Describable, error) {
	// If lastN is > 0, use the partial flatten image function.
	if lastN > 0 {
		return partialFlattenImage(old, repo, use, o, lastN)
	}

	digest, err := old.Digest()
	if err != nil {
		return nil, fmt.Errorf("getting old digest: %w", err)
	}
	m, err := old.Manifest()
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	cf, err := old.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("getting config: %w", err)
	}
	cf = cf.DeepCopy()

	oldHistory, err := json.Marshal(cf.History)
	if err != nil {
		return nil, fmt.Errorf("marshal history")
	}

	// Clear layer-specific config file information.
	cf.RootFS.DiffIDs = []v1.Hash{}
	cf.History = []v1.History{}
	cf.Created = v1.Time{Time: time.Now().UTC()}

	img, err := mutate.ConfigFile(empty.Image, cf)
	if err != nil {
		return nil, fmt.Errorf("mutating config: %w", err)
	}

	// TODO: Make compression configurable?
	layer := stream.NewLayer(mutate.Extract(old), stream.WithCompressionLevel(gzip.BestCompression))
	if err := remote.WriteLayer(repo, layer, o.Remote...); err != nil {
		return nil, fmt.Errorf("uploading layer: %w", err)
	}

	img, err = mutate.Append(img, mutate.Addendum{
		Layer: layer,
		History: v1.History{
			Created:   cf.Created,
			CreatedBy: fmt.Sprintf("%s flatten %s", use, digest),
			Comment:   string(oldHistory),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("appending layers: %w", err)
	}

	// Retain any annotations from the original image.
	if len(m.Annotations) != 0 {
		img = mutate.Annotations(img, m.Annotations).(v1.Image)
	}

	return img, nil
}

// partialFlattenImage flattens only the last N layers of an image, keeping the
// earlier layers intact. The final rootfs remains identical to the original
// image, but the layer topology is changed to have the first K-N layers plus a
// single merged layer for the last N layers.
func partialFlattenImage(old v1.Image, repo name.Repository, use string, o crane.Options, lastN int) (partial.Describable, error) {
	digest, err := old.Digest()
	if err != nil {
		return nil, fmt.Errorf("getting old digest: %w", err)
	}
	m, err := old.Manifest()
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	cf, err := old.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("getting config: %w", err)
	}
	// Keep a copy of the original history for later preservation.
	origHistory := cf.History
	origHistoryJSON, err := json.Marshal(origHistory)
	if err != nil {
		return nil, fmt.Errorf("marshal history: %w", err)
	}

	cf = cf.DeepCopy()

	layers, err := old.Layers()
	if err != nil {
		return nil, fmt.Errorf("getting layers: %w", err)
	}
	k := len(layers)
	if k == 0 {
		return old, nil
	}

	// If lastN is not in a sensible range, return an error.
	if lastN <= 0 || lastN > k {
		return nil, fmt.Errorf("last-n-layers must be between 1 and %d, got %d", k, lastN)
	}

	keepLayers := layers[:k-lastN]
	mergeLayers := layers[k-lastN:]

	// --- Scheme 1: compute real diffIDs for the mergeLayers and use them in a
	// temporary sub-image so mutate.Extract works correctly. ---
	diffIDs := make([]v1.Hash, 0, len(mergeLayers))
	for _, l := range mergeLayers {
		d, err := l.DiffID()
		if err != nil {
			return nil, fmt.Errorf("computing diffID of merge layer: %w", err)
		}
		diffIDs = append(diffIDs, d)
	}

	// Construct a minimal config for the sub-image with genuine diffIDs.
	subCfg := &v1.ConfigFile{}
	subCfg.RootFS.Type = "layers"
	subCfg.RootFS.DiffIDs = diffIDs

	subImg, err := mutate.ConfigFile(empty.Image, subCfg)
	if err != nil {
		return nil, fmt.Errorf("configuring sub image: %w", err)
	}

	// Append the actual merge layers to the sub-image.
	adds := make([]mutate.Addendum, 0, len(mergeLayers))
	for _, l := range mergeLayers {
		adds = append(adds, mutate.Addendum{Layer: l})
	}
	subImg, err = mutate.Append(subImg, adds...)
	if err != nil {
		return nil, fmt.Errorf("building sub image: %w", err)
	}

	// Extract the combined filesystem of the last N layers as a tar stream.
	mergedTar := mutate.Extract(subImg)
	defer mergedTar.Close()

	// Create a single merged layer from that tar stream.
	mergedLayer := stream.NewLayer(mergedTar, stream.WithCompressionLevel(gzip.BestCompression))

	// Upload the merged layer to the target repository (so it's available as a blob).
	if err := remote.WriteLayer(repo, mergedLayer, o.Remote...); err != nil {
		return nil, fmt.Errorf("uploading merged layer: %w", err)
	}

	// Now construct the new image config: preserve non-layer fields, reset
	// RootFS/History and set a new Created time. mutate.Append will derive the
	// final diffIDs when layers are appended.
	newCF := cf.DeepCopy()
	newCF.RootFS.DiffIDs = nil
	newCF.History = nil
	newCF.Created = v1.Time{Time: time.Now().UTC()}

	// Start a new image with that config.
	img, err := mutate.ConfigFile(empty.Image, newCF)
	if err != nil {
		return nil, fmt.Errorf("mutating config: %w", err)
	}

	// Prepare addenda: keep the leading layers unchanged (propagate their
	// history when possible), then append the merged layer with a history entry
	// that embeds the original history to avoid losing information.
	var addenda []mutate.Addendum

	// Naive history propagation: reuse as many leading history entries as we
	// have layers to keep. This is a pragmatic heuristic: precise mapping of
	// history to layers (considering empty layers) can be more involved.
	keepHistories := origHistory
	if len(keepHistories) > len(keepLayers) {
		keepHistories = keepHistories[:len(keepLayers)]
	}

	for i, l := range keepLayers {
		var h v1.History
		if i < len(keepHistories) {
			h = keepHistories[i]
		}
		addenda = append(addenda, mutate.Addendum{
			Layer:   l,
			History: h,
		})
	}

	mergedHistory := v1.History{
		Created:   newCF.Created,
		CreatedBy: fmt.Sprintf("%s flatten --last-n-layers=%d %s", use, lastN, digest),
		Comment:   string(origHistoryJSON),
	}
	addenda = append(addenda, mutate.Addendum{
		Layer:   mergedLayer,
		History: mergedHistory,
	})

	// Append all addenda to our new image.
	img, err = mutate.Append(img, addenda...)
	if err != nil {
		return nil, fmt.Errorf("appending layers: %w", err)
	}

	// Retain any annotations from the original image.
	if len(m.Annotations) != 0 {
		img = mutate.Annotations(img, m.Annotations).(v1.Image)
	}

	return img, nil
}
