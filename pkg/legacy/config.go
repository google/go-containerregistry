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

package legacy

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// LayerConfigFile is the configuration file that holds the metadata describing
// a v1 layer. See:
// https://github.com/moby/moby/blob/master/image/spec/v1.md
type LayerConfigFile struct {
	Architecture    string       `json:"architecture"`
	Author          string       `json:"author,omitempty"`
	Container       string       `json:"container,omitempty"`
	Created         v1.Time      `json:"created,omitempty"`
	Comment         string       `json:"comment,omitempty"`
	DockerVersion   string       `json:"docker_version,omitempty"`
	History         []v1.History `json:"history,omitempty"`
	OS              string       `json:"os"`
	RootFS          v1.RootFS    `json:"rootfs"`
	Config          v1.Config    `json:"config"`
	ContainerConfig v1.Config    `json:"container_config,omitempty"`
	OSVersion       string       `json:"osversion,omitempty"`
	ID              string       `json:"id,omitempty"`
	Parent          string       `json:"parent,omitempty"`
	Throwaway       bool         `json:"throwaway,omitempty"`
}
