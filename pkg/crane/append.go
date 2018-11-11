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

package crane

import (
	"archive/tar"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/cobra"
)

func init() { Root.AddCommand(NewCmdAppend()) }

func NewCmdAppend() *cobra.Command {
	var baseRef, newTag, newLayer, newLayerPath, outFile string
	appendCmd := &cobra.Command{
		Use:   "append",
		Short: "Append contents of a tarball to a remote image",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, args []string) {
			doAppend(baseRef, newTag, newLayer, newLayerPath, outFile)
		},
	}
	appendCmd.Flags().StringVarP(&baseRef, "base", "b", "", "Name of base image to append to")
	appendCmd.Flags().StringVarP(&newTag, "new_tag", "t", "", "Tag to apply to resulting image")
	appendCmd.Flags().StringVarP(&newLayer, "new_layer", "f", "", "Path to tarball to append to image")
	appendCmd.Flags().StringVarP(&newLayerPath, "new_layer_path", "p", "", "Path to directory containing files to append to image")
	appendCmd.Flags().StringVarP(&outFile, "output", "o", "", "Path to new tarball of resulting image")

	appendCmd.MarkFlagRequired("base")
	appendCmd.MarkFlagRequired("new_tag")
	return appendCmd
}

func doAppend(src, dst, tar, path, output string) {
	srcRef, err := name.ParseReference(src, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing reference %q: %v", src, err)
	}
	srcImage, err := remote.Image(srcRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		log.Fatalf("reading image %q: %v", srcRef, err)
	}

	dstTag, err := name.NewTag(dst, name.WeakValidation)
	if err != nil {
		log.Fatalf("parsing tag %q: %v", dst, err)
	}

	var layer v1.Layer
	if path != "" {
		layer = layerFromPath(path)
	} else if tar != "" {
		layer, err = tarball.LayerFromFile(tar)
		if err != nil {
			log.Fatalf("reading tar %q: %v", tar, err)
		}
	}

	image, err := mutate.AppendLayers(srcImage, layer)
	if err != nil {
		log.Fatalf("appending layer: %v", err)
	}

	if output != "" {
		if err := tarball.WriteToFile(output, dstTag, image); err != nil {
			log.Fatalf("writing output %q: %v", output, err)
		}
		return
	}

	dstAuth, err := authn.DefaultKeychain.Resolve(dstTag.Context().Registry)
	if err != nil {
		log.Fatalf("getting creds for %q: %v", dstTag, err)
	}

	if err := remote.Write(dstTag, image, dstAuth, http.DefaultTransport); err != nil {
		log.Fatalf("writing image %q: %v", dstTag, err)
	}
}

func layerFromPath(path string) v1.Layer {
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)
	go func() {
		// Stream the contents of the specified path to the tar.Writer
		pw.CloseWithError(func() error {
			filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() {
					return nil
				}
				// Write the file header.
				th, err := tar.FileInfoHeader(info, "")
				if err != nil {
					return err
				}
				th.Typeflag = tar.TypeRegA
				if err := tw.WriteHeader(th); err != nil {
					return err
				}

				f, err := os.Open(path)
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = io.Copy(tw, f)
				log.Println("added", path)
				return err
			})
			return nil
		}())
	}()
	return stream.NewLayer(pr)
}
