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

package mutate

// Flatten flattens the image to a single layer.
func Flatten(img v1.Image) (v1.Image, error) {
	newImage := empty.Image

	tar := Extract(img)
	layer, err := tarball.LayerFromReader(tar)
	if err != nil {
		return nil, fmt.Errorf("creating tarball: %w", err)
	}

	newImage, err = AppendLayers(newImage, layer)
	if err != nil {
		return nil, fmt.Errorf("appending layers: %w", err)
	}

	ocf, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("getting original config file: %w", err)
	}

	cf, err := newImage.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("setting config file: %w", err)
	}

	cfg := cf.DeepCopy()

	// Copy basic config over.
	cfg.Architecture = ocf.Architecture
	cfg.OS = ocf.OS
	cfg.OSVersion = ocf.OSVersion
	cfg.Config = ocf.Config

	return ConfigFile(newImage, cfg)
}