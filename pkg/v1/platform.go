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

package v1

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	platformSep = "/"
	jsonStart   = "{"
)

// Platform represents the target os/arch for an image.
type Platform struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
	Features     []string `json:"features,omitempty"`
}

// ParsePlatform builds a structured Platform object based on either:
// JSON string: {"os":"windows","architecture":"amd64","os.version":"10.0.14393.1066"}
// Inline short format: linux/amd64 or linux/arm64/v8
func ParsePlatform(p string) (*Platform, error) {
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, jsonStart) {
		var platform Platform
		err := json.Unmarshal([]byte(p), &platform)
		if err != nil {
			return nil, err
		}

		return &platform, nil
	}

	parts := strings.Split(p, platformSep)
	if len(parts) < 2 || len(parts) > 3 {
		return nil, fmt.Errorf("unable to parse platform: '%s', expected format is OS/ARCH(/VARIANT)", p)
	}

	platform := Platform{
		OS:           parts[0],
		Architecture: parts[1],
	}

	if len(parts) == 3 {
		platform.Variant = parts[2]
	}

	return &platform, nil
}

// Equals returns true if the given platform is semantically equivalent to this one.
// The order of Features and OSFeatures is not important.
func (p Platform) Equals(o Platform) bool {
	return p.OS == o.OS && p.Architecture == o.Architecture && p.Variant == o.Variant && p.OSVersion == o.OSVersion &&
		stringSliceEqualIgnoreOrder(p.OSFeatures, o.OSFeatures) && stringSliceEqualIgnoreOrder(p.Features, o.Features)
}

// stringSliceEqual compares 2 string slices and returns if their contents are identical.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, elm := range a {
		if elm != b[i] {
			return false
		}
	}
	return true
}

// stringSliceEqualIgnoreOrder compares 2 string slices and returns if their contents are identical, ignoring order
func stringSliceEqualIgnoreOrder(a, b []string) bool {
	a1, b1 := a[:], b[:]
	if a1 != nil && b1 != nil {
		sort.Strings(a1)
		sort.Strings(b1)
	}
	return stringSliceEqual(a1, b1)
}
