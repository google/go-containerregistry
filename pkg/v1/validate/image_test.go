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

package validate_test

import (
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestImage_EmptyConfig(t *testing.T) {
	img, err := random.Image(1024, 2)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}

	// Set empty config and empty config media type (BuildKit attestation format).
	img, err = mutate.ConfigFile(img, &v1.ConfigFile{})
	if err != nil {
		t.Fatalf("mutate.ConfigFile: %v", err)
	}
	img = mutate.ConfigMediaType(img, types.OCIEmptyJSON)

	if err := validate.Image(img); err != nil {
		t.Errorf("validate.Image() = %v, want nil", err)
	}

	if err := validate.Image(img, validate.Fast); err != nil {
		t.Errorf("validate.Image(Fast) = %v, want nil", err)
	}
}

func TestImage_DiffIDCount(t *testing.T) {
	base, err := random.Image(1024, 3)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	cs := []struct {
		name   string
		config *v1.ConfigFile
	}{
		{
			name: "fewer diffs",
			config: &v1.ConfigFile{
				RootFS: v1.RootFS{
					Type: "layers",
					DiffIDs: []v1.Hash{
						{Algorithm: "sha256", Hex: strings.Repeat("a", 64)},
					},
				},
			},
		},
		{
			name: "more diffs",
			config: &v1.ConfigFile{
				RootFS: v1.RootFS{
					Type: "layers",
					DiffIDs: []v1.Hash{
						{Algorithm: "sha256", Hex: strings.Repeat("a", 64)},
						{Algorithm: "sha256", Hex: strings.Repeat("b", 64)},
						{Algorithm: "sha256", Hex: strings.Repeat("c", 64)},
						{Algorithm: "sha256", Hex: strings.Repeat("d", 64)},
					},
				},
			},
		},
	}

	for _, tc := range cs {
		t.Run(tc.name, func(t *testing.T) {
			img, err := mutate.ConfigFile(base, &v1.ConfigFile{
				RootFS: v1.RootFS{
					Type: "layers",
					DiffIDs: []v1.Hash{
						{Algorithm: "sha256", Hex: strings.Repeat("a", 64)},
					},
				},
			})
			if err != nil {
				t.Fatalf("mutate.ConfigFile: %v", err)
			}

			err = validate.Image(img)
			if err == nil {
				t.Fatal("validate.Image() expected error, got nil")
			}
			if !strings.Contains(err.Error(), "mismatched number of diffids") {
				t.Errorf("expected 'mismatched number of diffids' in error, got %v", err)
			}
		})
	}
}

func TestImage_LeadingNonLayerBlob(t *testing.T) {
	nonLayer := static.NewLayer([]byte("in-toto attestation json"), types.MediaType("application/vnd.in-toto+json"))
	layer, err := random.Layer(1024, types.OCILayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}

	img, err := mutate.AppendLayers(empty.Image, nonLayer, layer)
	if err != nil {
		t.Fatalf("mutate.AppendLayers: %v", err)
	}

	diffID, err := layer.DiffID()
	if err != nil {
		t.Fatalf("layer.DiffID: %v", err)
	}

	img, err = mutate.ConfigFile(img, &v1.ConfigFile{
		RootFS: v1.RootFS{
			Type:    "layers",
			DiffIDs: []v1.Hash{diffID},
		},
	})
	if err != nil {
		t.Fatalf("mutate.ConfigFile: %v", err)
	}
	img = mutate.MediaType(img, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, types.OCIConfigJSON)

	if err := validate.Image(img); err != nil {
		t.Errorf("validate.Image() = %v, want nil", err)
	}
}
