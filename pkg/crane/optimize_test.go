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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
