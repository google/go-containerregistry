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
	"os/exec"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"
)

type LayerDefinition struct {
	Dirs []LayerDir `yaml:"dirs"`
}

type LayerDir struct {
	Name  string   `yaml:"targetDir"`
	Files []string `yaml:"files"`
}

type EnvVar struct {
	Name  string
	Value string
}

type CraneBuildConfig struct {
	BaseImage  string            `yaml:"baseImage"`
	Layers     []LayerDefinition `yaml:"layers"`
	Entrypoint string            `yaml:"entrypoint"`
	EnvVars    []EnvVar          `yaml:"env"`
}

func buildLayers(config *CraneBuildConfig) ([]string, error) {
	layers := make([]string, 0)
	for _, layerConfig := range config.Layers {
		layerPath, err := buildLayer(&layerConfig)
		if err != nil {
			return nil, err
		}
		layers = append(layers, layerPath)
	}
	return layers, nil
}

const (
	tarBaseCommand = "tar cf %s --group=0 --owner=0 --mtime='UTC 2019-01-01' --sort=name -C %s ."
)

func buildLayer(layerConfig *LayerDefinition) (string, error) {
	var err error
	layerTar, err := ioutil.TempFile("", "crane_layer*.tar")
	if err != nil {
		return "", fmt.Errorf("Failed to create tmp file for layer %w", err)
	}
	layerDir, err := ioutil.TempDir("", "crane_layer")
	if err != nil {
		return "", fmt.Errorf("Failed to create tmp dir for layer skeleton %w", err)
	}
	defer os.RemoveAll(layerDir)
	err = collectFiles(layerConfig, layerDir)
	if err != nil {
		return "", fmt.Errorf("Failed collecting files for layer %w", err)
	}
	tarCmd := fmt.Sprintf(tarBaseCommand, layerTar.Name(), layerDir)
	cmd := exec.Command("sh", "-c", tarCmd)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("%s\n", stdoutStderr)
		return "", fmt.Errorf("Failed creating tar for layer: %w", err)
	}
	return layerTar.Name(), nil
}

func collectFiles(layerConfig *LayerDefinition, targetDir string) error {
	for _, copyDir := range layerConfig.Dirs {
		target := filepath.Join(targetDir, copyDir.Name)
		err := os.MkdirAll(target, 0700)
		if err != nil {
			return fmt.Errorf("Failed to create dir %s (%+x)", target, err)
		}
		sources := strings.Join(copyDir.Files, " ")
		cmdString := fmt.Sprintf("cp -v %s %s", sources, target)
		cmd := exec.Command("sh", "-c", cmdString)
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("%s\n", stdoutStderr)
			return fmt.Errorf("Failed copying files (Command: %s) %w", cmdString, err)
		}
	}
	return nil
}

func buildImage(config *CraneBuildConfig, newLayers []string, options *[]crane.Option) (v1.Image, error) {
	var base v1.Image
	var err error

	if config.BaseImage == "" {
		logs.Warn.Printf("base unspecified, using empty image")
		base = empty.Image
	} else {
		base, err = crane.Pull(config.BaseImage, *options...)
		if err != nil {
			return nil, fmt.Errorf("pulling %s: %w", config.BaseImage, err)
		}
	}

	img, err := crane.Append(base, newLayers...)
	if err != nil {
		return nil, fmt.Errorf("appending %v: %w", newLayers, err)
	}

	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("Failed getting config: %w", err)
	}
	cfg = cfg.DeepCopy()

	cfg.Config.Env = createEnvStrings(config)
	cfg.Config.Entrypoint = []string{config.Entrypoint}

	// Mutate and write image.
	img, err = mutate.Config(img, cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("Failed mutating config: %w", err)
	}
	return img, nil
}

func createEnvStrings(config *CraneBuildConfig) []string {
	result := make([]string, 0)
	for _, item := range config.EnvVars {
		result = append(result, fmt.Sprintf("%s=%s", item.Name, item.Value))
	}
	return result
}

func saveImageToFile(img v1.Image, newTag string, outFile string) error {
	if err := crane.Save(img, newTag, outFile); err != nil {
		return fmt.Errorf("writing output %q: %w", outFile, err)
	}
	return nil
}

func saveImageToRepo(img v1.Image, newTag string) error {
	if err := crane.Push(img, newTag); err != nil {
		return fmt.Errorf("pushing image %s: %w", newTag, err)
	}
	ref, err := name.ParseReference(newTag)
	if err != nil {
		return fmt.Errorf("parsing reference %s: %w", newTag, err)
	}
	d, err := img.Digest()
	if err != nil {
		return fmt.Errorf("digest: %w", err)
	}
	fmt.Println(ref.Context().Digest(d.String()))
	return nil
}

func loadConfig(configFile string) (*CraneBuildConfig, error) {
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("unable to open config yaml file %s %w", configFile, err)
	}
	var result CraneBuildConfig
	if err = yaml.UnmarshalStrict(file, &result); err != nil {
		return nil, fmt.Errorf("failed unmarshalling yaml file %w", err)
	}
	return &result, nil
}

func NewCmdBuild(options *[]crane.Option) *cobra.Command {
	var configFile, newTag, outFile string
	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Use crane code and config yaml to build image",
		Long:  "Use crane code and config yaml to build image",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			config, err := loadConfig(configFile)
			if err != nil {
				return err
			}
			newLayers, err := buildLayers(config)
			if err != nil {
				return err
			}
			img, err := buildImage(config, newLayers, options)
			if outFile != "" {
				return saveImageToFile(img, newTag, outFile)
			}
			return saveImageToRepo(img, newTag)
		},
	}
	buildCmd.Flags().StringVar(&configFile, "config", "", "path to config file")
	buildCmd.Flags().StringVarP(&newTag, "new_tag", "t", "", "Tag to apply to resulting image")
	buildCmd.Flags().StringVarP(&outFile, "output", "o", "", "Path to new tarball of resulting image")

	buildCmd.MarkFlagRequired("config")
	buildCmd.MarkFlagRequired("new_tag")
	return buildCmd
}
