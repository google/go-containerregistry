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
	}{{
		v1.Platform{},
		"",
	}, {
		v1.Platform{OS: "linux"},
		"linux",
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64"},
		"linux/amd64",
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64", Variant: "v7"},
		"linux/amd64/v7",
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3.4"},
		"linux/amd64:1.2.3.4",
	}, {
		v1.Platform{OS: "linux", Architecture: "amd64", OSVersion: "1.2.3.4", OSFeatures: []string{"a", "b"}, Features: []string{"c", "d"}},
		"linux/amd64:1.2.3.4",
	}} {
		if got := c.plat.String(); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}

		if len(c.plat.OSFeatures) > 0 || len(c.plat.Features) > 0 {
			// If these values are set, roundtripping back to the
			// Platform will be lossy, and we expect that.
			continue
		}

		back, err := v1.ParsePlatform(c.plat.String())
		if err != nil {
			t.Errorf("ParsePlatform(%q): %v", c.plat, err)
		}
		if d := cmp.Diff(&c.plat, back); d != "" {
			t.Errorf("ParsePlatform(%q) diff:\n%s", c.plat.String(), d)
		}
	}

	// Known bad examples.
	for _, s := range []string{
		"linux/amd64/v7/s9", // too many slashes
	} {
		got, err := v1.ParsePlatform(s)
		if err == nil {
			t.Errorf("ParsePlatform(%q) wanted error; got %v", s, got)
		}
	}
}

func TestPlatformEquals(t *testing.T) {
	tests := []struct {
		a, b  v1.Platform
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

func TestPlatformSatisfies(t *testing.T) {
	tests := []struct {
		have, spec v1.Platform
		sat        bool
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
		true,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", OSVersion: "5.0"},
		v1.Platform{Architecture: "amd64", OS: "linux", OSVersion: "3.6"},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", Variant: "pios"},
		v1.Platform{Architecture: "amd64", OS: "linux"},
		true,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", Variant: "pios"},
		v1.Platform{Architecture: "amd64", OS: "linux", Variant: "ubuntu"},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", Variant: "pios"},
		v1.Platform{Architecture: "amd64", OS: "linux", Variant: "pios"},
		true,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux"},
		v1.Platform{Architecture: "amd64", OS: "linux", OSFeatures: []string{"a", "b"}},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", OSFeatures: []string{"a", "b"}},
		v1.Platform{Architecture: "amd64", OS: "linux"},
		true,
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
		v1.Platform{Architecture: "amd64", OS: "linux"},
		v1.Platform{Architecture: "amd64", OS: "linux", Features: []string{"a", "b"}},
		false,
	}, {
		v1.Platform{Architecture: "amd64", OS: "linux", Features: []string{"a", "b"}},
		v1.Platform{Architecture: "amd64", OS: "linux"},
		true,
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
		if sat := tt.have.Satisfies(tt.spec); sat != tt.sat {
			t.Errorf("%d: mismatched was %v expected %v; original (-want +got) %s", i, sat, tt.sat, cmp.Diff(tt.have, tt.spec))
		}
	}
}
