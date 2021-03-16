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

package crane

import (
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestStringSet(t *testing.T) {
	for _, tc := range []struct {
		lhs    []string
		rhs    []string
		result []string
	}{{
		lhs:    []string{},
		rhs:    []string{},
		result: []string{},
	}, {
		lhs:    []string{"a"},
		rhs:    []string{},
		result: []string{},
	}, {
		lhs:    []string{},
		rhs:    []string{"a"},
		result: []string{},
	}, {
		lhs:    []string{"a", "b", "c"},
		rhs:    []string{"a", "b", "c"},
		result: []string{"a", "b", "c"},
	}, {
		lhs:    []string{"a", "b"},
		rhs:    []string{"a"},
		result: []string{"a"},
	}, {
		lhs:    []string{"a"},
		rhs:    []string{"a", "b"},
		result: []string{"a"},
	}} {
		got := newStringSet(tc.lhs).Intersection(newStringSet(tc.rhs))
		want := newStringSet(tc.result)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%v.intersect(%v) (-want +got): %s", tc.lhs, tc.rhs, diff)
		}

		less := func(a, b string) bool {
			return strings.Compare(a, b) <= -1
		}
		if diff := cmp.Diff(tc.result, got.List(), cmpopts.SortSlices(less)); diff != "" {
			t.Errorf("%v.List() (-want +got): = %v", tc.result, diff)
		}
	}
}

func TestOptimize(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	imgs := []mutate.IndexAddendum{}
	for _, plat := range []string{
		"linux/amd64",
		"linux/arm",
	} {
		img, err := Image(map[string][]byte{
			"unimportant":  []byte(strings.Repeat("deadbeef", 128)),
			"important":    []byte(`abc`),
			"platform.txt": []byte(plat),
		})
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(plat, "/")
		imgs = append(imgs, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{
					OS:           parts[0],
					Architecture: parts[1],
				},
			},
		})
	}

	idx := mutate.AppendManifests(empty.Index, imgs...)

	slow := path.Join(u.Host, "slow")
	fast := path.Join(u.Host, "fast")

	ref, err := name.ParseReference(slow)
	if err != nil {
		t.Fatal(err)
	}

	if err := remote.WriteIndex(ref, idx); err != nil {
		t.Fatal(err)
	}

	if err := Optimize(slow, fast, []string{"important"}); err != nil {
		t.Fatal(err)
	}

	if err := Optimize(slow, fast, []string{"important"}, WithPlatform(imgs[1].Platform)); err != nil {
		t.Fatal(err)
	}

	// Compare optimize WithPlatform path to optimizing just an image.
	got, err := Digest(fast)
	if err != nil {
		t.Fatal(err)
	}

	dig, err := Digest(slow, WithPlatform(imgs[1].Platform))
	if err != nil {
		t.Fatal(err)
	}

	slowImgRef := slow + "@" + dig
	if err := Optimize(slowImgRef, fast, []string{"important"}, WithPlatform(imgs[1].Platform)); err != nil {
		t.Fatal(err)
	}

	want, err := Digest(fast)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("Optimize(WithPlatform) != Optimize(bydigest): %q != %q", got, want)
	}

	for i, ref := range []string{slow, slow, slowImgRef} {
		opts := []Option{}
		// Silly, but use WithPlatform to get some more coverage.
		if i != 0 {
			opts = []Option{WithPlatform(imgs[1].Platform)}
		}
		dig, err := Digest(ref, opts...)
		if err != nil {
			t.Errorf("Digest(%q): %v", ref, err)
			continue
		}
		// Make sure we fail if there's a missing file in the optimize set
		// Use the image digest because it's ~impossible to exist in img.
		if err := Optimize(ref, fast, []string{dig}, opts...); err == nil {
			t.Errorf("Optimize(%q, prioritize=%q): got nil, want err", ref, dig)
		} else if !strings.Contains(err.Error(), dig) {
			// Make sure this contains the missing file (dig)
			t.Errorf("Optimize(%q) error should contain %q, got: %v", ref, dig, err)
		}
	}
}
