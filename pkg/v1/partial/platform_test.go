// Copyright 2020 Google LLC All Rights Reserved.
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

package partial

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// TestMatchesPlatform runs test cases on the matchesPlatform function which verifies
// whether the given platform can run on the required platform by checking the
// compatibility of architecture, OS, OS version, OS features, variant and features.
func TestMatchesPlatform(t *testing.T) {
	t.Parallel()
	tests := []struct {
		// want is the expected return value from matchesPlatform
		// when the given platform is 'given' and the required platform is 'required'.
		given    v1.Platform
		required v1.Platform
		want     bool
	}{{ // The given & required platforms are identical. matchesPlatform expected to return true.
		given: v1.Platform{
			Architecture: "amd64",
			OS:           "linux",
			OSVersion:    "10.0.10586",
			OSFeatures:   []string{"win32k"},
			Variant:      "armv6l",
			Features:     []string{"sse4"},
		},
		required: v1.Platform{
			Architecture: "amd64",
			OS:           "linux",
			OSVersion:    "10.0.10586",
			OSFeatures:   []string{"win32k"},
			Variant:      "armv6l",
			Features:     []string{"sse4"},
		},
		want: true,
	},
		{ // OS and Architecture must exactly match. matchesPlatform expected to return false.
			given: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			required: v1.Platform{
				Architecture: "amd64",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win32k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			want: false,
		},
		{ // OS version must exactly match
			given: v1.Platform{
				Architecture: "amd64",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			required: v1.Platform{
				Architecture: "amd64",
				OS:           "linux",
				OSVersion:    "10.0.10587",
				OSFeatures:   []string{"win64k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			want: false,
		},
		{ // OS Features must exactly match. matchesPlatform expected to return false.
			given: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			required: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win32k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			want: false,
		},
		{ // Variant must exactly match. matchesPlatform expected to return false.
			given: v1.Platform{
				Architecture: "amd64",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			required: v1.Platform{
				Architecture: "amd64",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k"},
				Variant:      "armv7l",
				Features:     []string{"sse4"},
			},
			want: false,
		},
		{ // OS must exactly match, and is case sensative. matchesPlatform expected to return false.
			given: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			required: v1.Platform{
				Architecture: "arm",
				OS:           "LinuX",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			want: false,
		},
		{ // OSVersion and Variant are specified in given but not in required.
			// matchesPlatform expected to return true.
			given: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			required: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "",
				OSFeatures:   []string{"win64k"},
				Variant:      "",
				Features:     []string{"sse4"},
			},
			want: true,
		},
		{ // Ensure the optional field OSVersion & Variant match exactly if specified as required.
			given: v1.Platform{
				Architecture: "amd64",
				OS:           "linux",
				OSVersion:    "",
				OSFeatures:   []string{},
				Variant:      "",
				Features:     []string{},
			},
			required: v1.Platform{
				Architecture: "amd64",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win32k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			want: false,
		},
		{ // Checking subset validity when required less features than given features.
			// matchesPlatform expected to return true.
			given: v1.Platform{
				Architecture: "",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win32k"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			required: v1.Platform{
				Architecture: "",
				OS:           "linux",
				OSVersion:    "",
				OSFeatures:   []string{},
				Variant:      "",
				Features:     []string{},
			},
			want: true,
		},
		{ // Checking subset validity when required features are subset of given features.
			// matchesPlatform expected to return true.
			given: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k", "f1", "f2"},
				Variant:      "",
				Features:     []string{"sse4", "f1"},
			},
			required: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k"},
				Variant:      "",
				Features:     []string{"sse4"},
			},
			want: true,
		},
		{ // Checking subset validity when some required features is not subset of given features.
			// matchesPlatform expected to return false.
			given: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k", "f1", "f2"},
				Variant:      "",
				Features:     []string{"sse4", "f1"},
			},
			required: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k"},
				Variant:      "",
				Features:     []string{"sse4", "f2"},
			},
			want: false,
		},
		{ // Checking subset validity when OS features not required,
			// and required features is indeed a subset of given features.
			// matchesPlatform expected to return true.
			given: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{"win64k", "f1", "f2"},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			required: v1.Platform{
				Architecture: "arm",
				OS:           "linux",
				OSVersion:    "10.0.10586",
				OSFeatures:   []string{},
				Variant:      "armv6l",
				Features:     []string{"sse4"},
			},
			want: true,
		},
	}

	for _, test := range tests {
		got := matchesPlatform(test.given, test.required)
		if got != test.want {
			t.Errorf("matchesPlatform(%v, %v); got %v, want %v", test.given, test.required, got, test.want)
		}
	}

}
