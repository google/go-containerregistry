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
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"
)

// NewCmdAnnotate creates a new cobra.Command for the annotate subcommand.
func NewCmdAnnotate(options *[]crane.Option) *cobra.Command {
	var annotations map[string]string
	var newRef string

	annotateCmd := &cobra.Command{
		Use:   "annotate",
		Short: "Modify image or index annotations. The manifest is updated there on the registry.",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			ref := args[0]

			desc, err := crane.Head(ref, *options...)
			if err != nil {
				return err
			}

			if newRef == "" {
				newRef = ref
			}

			if desc.MediaType.IsIndex() {
				d, err := crane.Get(ref, *options...)
				if err != nil {
					return err
				}
				idx, err := d.ImageIndex()
				if err != nil {
					return err
				}
				annotated := mutate.Annotations(idx, annotations).(v1.ImageIndex)
				err = crane.PushIndex(annotated, newRef, *options...)
				if err != nil {
					return err
				}
				fmt.Println("Index pushed with annotations.")
			} else if desc.MediaType.IsImage() {
				img, err := crane.Pull(ref, *options...)
				if err != nil {
					return err
				}
				annotated := mutate.Annotations(img, annotations).(v1.Image)
				err = crane.Push(annotated, newRef, *options...)
				if err != nil {
					return err
				}
				fmt.Println("Image pushed with annotations.")
			} else {
				return fmt.Errorf("unsupported manifest type only indexes and images are currently supported: %s", desc.ArtifactType)
			}

			return nil
		},
	}
	annotateCmd.Flags().StringToStringVarP(&annotations, "annotation", "a", nil, "New annotations to add")
	annotateCmd.Flags().StringVarP(&newRef, "tag", "t", "", "New tag reference to apply to annotated image/index. If not provided, push by digest to the original repository.")
	return annotateCmd
}
