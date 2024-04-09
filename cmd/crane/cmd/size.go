// Copyright 2024 Google LLC All Rights Reserved.
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
	"math"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
)

// NewCmdSize creates a new cobra.Command for the size subcommand.
func NewCmdSize() *cobra.Command {
	var platform string
	var human bool
	sizeCmd := &cobra.Command{
		Use:   "size",
		Short: "Return the size of an image",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ref, err := name.ParseReference(args[0])
			if err != nil {
				return err
			}

			opts := []remote.Option{remote.WithAuthFromKeychain(authn.DefaultKeychain)}
			if platform != "" {
				p, err := v1.ParsePlatform(platform)
				if err != nil {
					return err
				}
				opts = append(opts, remote.WithPlatform(*p))
			}

			desc, err := remote.Get(ref, opts...)
			if err != nil {
				return err
			}
			var sz int64
			if platform != "" {
				isz, err := imageSize(desc.Image())
				if err != nil {
					return err
				}
				sz = isz
			} else {
				idx, err := desc.ImageIndex()
				if err != nil {
					return err
				}

				mf, err := idx.IndexManifest()
				if err != nil {
					return err
				}
				for _, m := range mf.Manifests {
					isz, err := imageSize(remote.Image(ref.Context().Digest(m.Digest.String()), remote.WithAuthFromKeychain(authn.DefaultKeychain)))
					if err != nil {
						return err
					}
					sz += isz
				}
			}

			if human {
				fmt.Printf("%s\n", ibytes(uint64(sz)))
			} else {
				fmt.Printf("%d\n", sz)
			}
			return nil
		},
	}
	sizeCmd.Flags().StringVar(&platform, "platform", "", "The platform to use when pulling the image. If empty, size of all platforms' blobs.")
	sizeCmd.Flags().BoolVar(&human, "human", false, "If true, print human-readable sizes.")
	return sizeCmd
}

func imageSize(img v1.Image, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	ls, err := img.Layers()
	if err != nil {
		return 0, err
	}
	var sz int64
	for i, l := range ls {
		lsz, err := l.Size()
		if err != nil {
			return 0, fmt.Errorf("getting layer %d size: %w", i, err)
		}
		sz += lsz
	}
	mf, err := img.Manifest()
	if err != nil {
		return 0, fmt.Errorf("getting manifest: %w", err)
	}
	sz += mf.Config.Size
	return sz, nil
}

// Copied from https://github.com/dustin/go-humanize/blob/961771c7ab9992c55cd100b0562246e970925856/bytes.go

func logn(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}

func humanateBytes(s uint64, base float64, sizes []string) string {
	if s < 10 {
		return fmt.Sprintf("%d B", s)
	}
	e := math.Floor(logn(float64(s), base))
	suffix := sizes[int(e)]
	val := math.Floor(float64(s)/math.Pow(base, e)*10+0.5) / 10
	f := "%.0f %s"
	if val < 10 {
		f = "%.1f %s"
	}

	return fmt.Sprintf(f, val, suffix)
}

// ibytes produces a human readable representation of an IEC size.
//
// ibytes(82854982) -> 79 MiB
func ibytes(s uint64) string {
	sizes := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	return humanateBytes(s, 1024, sizes)
}
