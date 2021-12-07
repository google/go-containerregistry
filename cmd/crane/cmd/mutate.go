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
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"
)

// NewCmdMutate creates a new cobra.Command for the mutate subcommand.
func NewCmdMutate(options *[]crane.Option) *cobra.Command {
	var labels map[string]string
	var annotations map[string]string
	var entrypoint, cmd []string
	var envVars map[string]string
	var newLayers []string

	var newRef string

	mutateCmd := &cobra.Command{
		Use:   "mutate",
		Short: "Modify image labels and annotations. The container must be pushed to a registry, and the manifest is updated there.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			// Pull image and get config.
			ref := args[0]

			if len(annotations) != 0 {
				desc, err := crane.Head(ref, *options...)
				if err != nil {
					return err
				}
				if desc.MediaType.IsIndex() {
					return errors.New("mutating annotations on an index is not yet supported")
				}
			}

			img, err := crane.Pull(ref, *options...)
			if err != nil {
				return fmt.Errorf("pulling %s: %w", ref, err)
			}
			if len(newLayers) != 0 {
				img, err = crane.Append(img, newLayers...)
				if err != nil {
					return fmt.Errorf("appending %v: %w", newLayers, err)
				}
			}
			cfg, err := img.ConfigFile()
			if err != nil {
				return err
			}
			cfg = cfg.DeepCopy()

			// Set labels.
			if cfg.Config.Labels == nil {
				cfg.Config.Labels = map[string]string{}
			}

			if err := validateKeyVals(labels); err != nil {
				return err
			}

			for k, v := range labels {
				cfg.Config.Labels[k] = v
			}

			if err := validateKeyVals(annotations); err != nil {
				return err
			}

			// set envvars if specified
			if err := setEnvVars(&cfg.Config, envVars); err != nil {
				return err
			}

			// Set entrypoint.
			if len(entrypoint) > 0 {
				cfg.Config.Entrypoint = entrypoint
				cfg.Config.Cmd = nil // This matches Docker's behavior.
			}

			// Set cmd.
			if len(cmd) > 0 {
				cfg.Config.Cmd = cmd
			}

			// Mutate and write image.
			img, err = mutate.Config(img, cfg.Config)
			if err != nil {
				return fmt.Errorf("mutating config: %w", err)
			}

			img = mutate.Annotations(img, annotations).(v1.Image)

			// If the new ref isn't provided, write over the original image.
			// If that ref was provided by digest (e.g., output from
			// another crane command), then strip that and push the
			// mutated image by digest instead.
			if newRef == "" {
				newRef = ref
			}
			digest, err := img.Digest()
			if err != nil {
				return fmt.Errorf("digesting new image: %w", err)
			}
			r, err := name.ParseReference(newRef)
			if err != nil {
				return fmt.Errorf("parsing %s: %w", newRef, err)
			}
			if _, ok := r.(name.Digest); ok {
				newRef = r.Context().Digest(digest.String()).String()
			}
			if err := crane.Push(img, newRef, *options...); err != nil {
				return fmt.Errorf("pushing %s: %w", newRef, err)
			}
			fmt.Println(r.Context().Digest(digest.String()))
			return nil
		},
	}
	mutateCmd.Flags().StringToStringVarP(&annotations, "annotation", "a", nil, "New annotations to add")
	mutateCmd.Flags().StringToStringVarP(&labels, "label", "l", nil, "New labels to add")
	mutateCmd.Flags().StringToStringVarP(&envVars, "env", "e", nil, "New envvar to add")
	mutateCmd.Flags().StringSliceVar(&entrypoint, "entrypoint", nil, "New entrypoint to set")
	mutateCmd.Flags().StringSliceVar(&cmd, "cmd", nil, "New cmd to set")
	mutateCmd.Flags().StringVarP(&newRef, "tag", "t", "", "New tag to apply to mutated image. If not provided, push by digest to the original image repository.")
	mutateCmd.Flags().StringSliceVarP(&newLayers, "new_layer", "f", []string{}, "Path to tarball to append to image")
	return mutateCmd
}

// validateKeyVals ensures no values are empty, returns error if they are
func validateKeyVals(kvPairs map[string]string) error {
	for label, value := range kvPairs {
		if value == "" {
			return fmt.Errorf("parsing label %q, value is empty", label)
		}
	}
	return nil
}

// setEnvVars override envvars in a config
func setEnvVars(cfg *v1.Config, envVars map[string]string) error {
	existing := map[string]string{}
	for _, kv := range cfg.Env {
		split := strings.SplitN(kv, "=", 2)
		if len(split) != 2 {
			return fmt.Errorf("Invalid key value pair in config: %s", kv)
		}
		existing[split[0]] = split[1]
	}
	for k, v := range envVars {
		existing[k] = v
	}
	cfg.Env = make([]string, 0, len(existing))
	for k, v := range existing {
		cfg.Env = append(cfg.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return nil
}
