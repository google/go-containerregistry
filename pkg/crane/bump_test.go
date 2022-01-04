// Copyright 2019 Google LLC All Rights Reserved.
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

package crane_test

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

func TestCraneBump(t *testing.T) {
	for i, tc := range []struct {
		oldTags []string
		newTags []string
		newTag  string
		want    []string
	}{{
		oldTags: []string{"v0.0.1"},
		newTags: []string{"new"},
		newTag:  "v0.0.2",
		want:    []string{"v0.0.2", "v0.0", "v0", "latest"},
	}, {
		oldTags: []string{"v0.0.1", "v0.0", "v0", "latest"},
		newTags: []string{"new"},
		newTag:  "v0.0.2",
		want:    []string{"v0.0.2", "v0.0", "v0", "latest"},
	}, {
		oldTags: []string{"v0.1.1", "v0.1", "v0.2.3", "v0.2", "v0", "latest"},
		newTags: []string{"new"},
		newTag:  "v0.1.2",
		want:    []string{"v0.1.2", "v0.1"},
	}} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// Set up a fake registry.
			s := httptest.NewServer(registry.New())
			defer s.Close()
			u, err := url.Parse(s.URL)
			if err != nil {
				t.Fatal(err)
			}

			repo := fmt.Sprintf("%s/test/bump", u.Host)

			oldImg, err := random.Image(1024, 5)
			if err != nil {
				t.Fatal(err)
			}

			newImg, err := random.Image(1024, 5)
			if err != nil {
				t.Fatal(err)
			}

			for _, tag := range tc.oldTags {
				if err := crane.Push(oldImg, repo+":"+tag); err != nil {
					t.Fatal(err)
				}
			}
			for _, tag := range tc.newTags {
				if err := crane.Push(newImg, repo+":"+tag); err != nil {
					t.Fatal(err)
				}
			}

			logs.Progress.SetOutput(os.Stderr)
			got, err := crane.Bump(repo+":"+"new", tc.newTag)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(got), len(tc.want); got != want {
				t.Fatalf("wanted %d got %d", want, got)
			}
			for _, got := range got {
				t.Log(got)
			}
		})
	}
}
