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

package v1_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func TestPlatformString(t *testing.T) {
	for _, c := range []struct {
		plat v1.Platform
		want string
	}{
		{v1.Platform{}, ""},
		{v1.Platform{OS: "linux"}, "linux"},
		{v1.Platform{OS: "linux", Architecture: "amd64"}, "linux/amd64"},
		{v1.Platform{OS: "linux", Architecture: "amd64", Variant: "v7"}, "linux/amd64/v7"},
		{v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3.4"}, "linux/amd64:1.2.3.4"},
		{v1.Platform{OS: "linux", Architecture: "amd64", OSFeatures: []string{"a", "b"}}, "linux/amd64 (osfeatures=a,b)"},
		{v1.Platform{OS: "linux", Architecture: "amd64", OSFeatures: []string{"a", "b"}, Features: []string{"c", "d"}}, "linux/amd64 (osfeatures=a,b) (features=c,d)"},
	} {
		if got := c.plat.String(); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

func TestPlatformEquals(t *testing.T) {
	tests := []struct {
		a     v1.Platform
		b     v1.Platform
		equal bool
	}{{
		v1.Platform{Architecture: "amd64", OS: "linux"},
		v1.Platform{Architecture: "amd64", OS: "linux"},
		true,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux"},
		v1.Platform{Architecture: "arm64", OS: "linux"},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux"},
		v1.Platform{Architecture: "amd64", OS: "darwin"},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", OSVersion: "5.0"},
		v1.Platform{Architecture: "amd64", OS: "linux"},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", OSVersion: "5.0"},
		v1.Platform{Architecture: "amd64", OS: "linux", OSVersion: "3.6"},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", Variant: "pios"},
		v1.Platform{Architecture: "amd64", OS: "linux"},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", Variant: "pios"},
		v1.Platform{Architecture: "amd64", OS: "linux", Variant: "ubuntu"},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", Variant: "pios"},
		v1.Platform{Architecture: "amd64", OS: "linux", Variant: "pios"},
		true,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", OSFeatures: []string{"a", "b"}},
		v1.Platform{Architecture: "amd64", OS: "linux"},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", OSFeatures: []string{"a", "b"}},
		v1.Platform{Architecture: "amd64", OS: "linux", OSFeatures: []string{"a", "b"}},
		true,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", OSFeatures: []string{"a", "b"}},
		v1.Platform{Architecture: "amd64", OS: "linux", OSFeatures: []string{"ac", "bd"}},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", OSFeatures: []string{"a", "b"}},
		v1.Platform{Architecture: "amd64", OS: "linux", OSFeatures: []string{"b", "a"}},
		true,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", Features: []string{"a", "b"}},
		v1.Platform{Architecture: "amd64", OS: "linux"},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", Features: []string{"a", "b"}},
		v1.Platform{Architecture: "amd64", OS: "linux", Features: []string{"a", "b"}},
		true,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", Features: []string{"a", "b"}},
		v1.Platform{Architecture: "amd64", OS: "linux", Features: []string{"ac", "bd"}},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", Features: []string{"a", "b"}},
		v1.Platform{Architecture: "amd64", OS: "linux", Features: []string{"b", "a"}},
		true,
	}}
	for i, tt := range tests {
		if equal := tt.a.Equals(tt.b); equal != tt.equal {
			t.Errorf("%d: mismatched was %v expected %v; original (-want +got) %s", i, equal, tt.equal, cmp.Diff(tt.a, tt.b))
		}
	}
}

func TestPlatformFuzzyEquals(t *testing.T) {
	tests := []struct {
		a     v1.Platform
		b     v1.Platform
		equal bool
	}{{
		v1.Platform{OS: "linux"},
		v1.Platform{OS: "linux", Architecture: "amd64"},
		true,
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64"},
		v1.Platform{OS: "linux", Architecture: "amd64", Variant: "v7"},
		true,
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64"},
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3.4"},
		true,
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1"},
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3.4"},
		true,
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2"},
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3.4"},
		true,
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3"},
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3.4"},
		true,
	}, {
		v1.Platform{OS: "linux"},
		v1.Platform{OS: "windows"},
		false,
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64"},
		v1.Platform{OS: "linux", Architecture: "arm64"},
		false,
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64", Variant: "v6"},
		v1.Platform{OS: "linux", Architecture: "amd64", Variant: "v7"},
		false,
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1"},
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "2"},
		false,
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3.4"},
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3.6"},
		false,
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3.4.5"},
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3.4"},
		false,
	}}
	for i, tt := range tests {
		if equal := tt.a.FuzzyEquals(tt.b); equal != tt.equal {
			t.Errorf("%d: mismatched was %v expected %v; original (-want +got) %s", i, equal, tt.equal, cmp.Diff(tt.a, tt.b))
		}
	}
}
