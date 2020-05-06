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

package gcrane

import (
	"encoding/json"
	"net/http"
	"path"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/partial"
)

func mustRepo(s string) name.Repository {
	repo, err := name.NewRepository(s)
	if err != nil {
		panic(err)
	}
	return repo
}

type fakeGCR struct {
	h     http.Handler
	repos map[string]google.Tags
	t     *testing.T
}

func (gcr *fakeGCR) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	gcr.t.Logf("%s %s", r.Method, r.URL)
	if strings.HasPrefix(r.URL.Path, "/v2/") && strings.HasSuffix(r.URL.Path, "/tags/list") {
		repo := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v2/"), "/tags/list")
		if tags, ok := gcr.repos[repo]; !ok {
			w.WriteHeader(http.StatusNotFound)
		} else {
			gcr.t.Logf("%+v", tags)
			json.NewEncoder(w).Encode(tags)
		}
	} else {
		gcr.h.ServeHTTP(w, r)
	}
}

func newFakeGCR(stuff map[name.Reference]partial.Describable, t *testing.T) (*fakeGCR, error) {
	h := registry.New()

	repos := make(map[string]google.Tags)

	for ref, thing := range stuff {
		repo := ref.Context().RepositoryStr()
		tags, ok := repos[repo]
		if !ok {
			tags = google.Tags{
				Name:     repo,
				Children: []string{},
			}
		}

		// Populate the "child" field.
		for parentPath := repo; parentPath != "."; parentPath = path.Dir(parentPath) {
			child, parent := path.Base(parentPath), path.Dir(parentPath)
			tags, ok := repos[parent]
			if !ok {
				tags = google.Tags{}
			}
			for _, c := range repos[parent].Children {
				if c == child {
					break
				}
			}
			tags.Children = append(tags.Children, child)
			repos[parent] = tags
		}

		// Populate the "manifests" and "tags" field.
		d, err := thing.Digest()
		if err != nil {
			return nil, err
		}
		mt, err := thing.MediaType()
		if err != nil {
			return nil, err
		}
		if tags.Manifests == nil {
			tags.Manifests = make(map[string]google.ManifestInfo)
		}
		mi, ok := tags.Manifests[d.String()]
		if !ok {
			mi = google.ManifestInfo{
				MediaType: string(mt),
				Tags:      []string{},
			}
		}
		if tag, ok := ref.(name.Tag); ok {
			tags.Tags = append(tags.Tags, tag.Identifier())
			mi.Tags = append(mi.Tags, tag.Identifier())
		}
		tags.Manifests[d.String()] = mi
		repos[repo] = tags
	}

	return &fakeGCR{h: h, t: t, repos: repos}, nil
}
