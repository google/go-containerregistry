// Copyright 2022 Google LLC All Rights Reserved.
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
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type platformValue struct {
	platform *v1.Platform
}

func (pv *platformValue) Set(platform string) error {
	if platform == "all" {
		return nil
	}

	p, err := v1.PlatformFromString(platform)
	if err != nil {
		return err
	}
	pv.platform = p
	return nil
}

func (pv *platformValue) String() string {
	if pv == nil || pv.platform == nil {
		return "all"
	}
	return pv.platform.String()
}

func (pv *platformValue) Type() string {
	return "platform"
}
