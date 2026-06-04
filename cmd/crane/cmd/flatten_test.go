// Copyright 2026 Google LLC All Rights Reserved.
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
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestFlattenImagePreservesMediaType(t *testing.T) {
	for _, mt := range []types.MediaType{
		types.OCIManifestSchema1,
		types.DockerManifestSchema2,
	} {
		t.Run(string(mt), func(t *testing.T) {
			s := httptest.NewServer(registry.New())
			defer s.Close()
			u, err := url.Parse(s.URL)
			if err != nil {
				t.Fatal(err)
			}

			repo, err := name.NewRepository(fmt.Sprintf("%s/test/flatten", u.Host))
			if err != nil {
				t.Fatal(err)
			}

			img, err := random.Image(1024, 2)
			if err != nil {
				t.Fatalf("random.Image: %v", err)
			}
			img = mutate.MediaType(img, mt)

			flat, err := flattenImage(img, repo, "crane", crane.GetOptions())
			if err != nil {
				t.Fatalf("flattenImage: %v", err)
			}

			flatImg, ok := flat.(v1.Image)
			if !ok {
				t.Fatalf("flattenImage returned %T, want v1.Image", flat)
			}

			got, err := flatImg.MediaType()
			if err != nil {
				t.Fatalf("MediaType: %v", err)
			}
			if got != mt {
				t.Errorf("flattened media type = %q, want %q", got, mt)
			}
		})
	}
}
