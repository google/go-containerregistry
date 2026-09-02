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

func TestIndex_WithAttestationChild(t *testing.T) {
	// Create base image
	img, err := random.Image(1024, 2)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	img = mutate.MediaType(img, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, types.OCIConfigJSON)
	cf, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("img.ConfigFile: %v", err)
	}
	cf = cf.DeepCopy()
	cf.OS = "linux"
	cf.Architecture = "amd64"
	img, err = mutate.ConfigFile(img, cf)
	if err != nil {
		t.Fatalf("mutate.ConfigFile: %v", err)
	}

	// Create attestation manifest (in-toto layer, empty config)
	attestationLayer := static.NewLayer([]byte(`{"_type": "https://in-toto.io/Statement/v0.1"}`), types.MediaType("application/vnd.in-toto+json"))
	attImg, err := mutate.AppendLayers(empty.Image, attestationLayer)
	if err != nil {
		t.Fatalf("mutate.AppendLayers: %v", err)
	}
	attImg, err = mutate.ConfigFile(attImg, &v1.ConfigFile{})
	if err != nil {
		t.Fatalf("mutate.ConfigFile: %v", err)
	}
	attImg = mutate.MediaType(attImg, types.OCIManifestSchema1)
	attImg = mutate.ConfigMediaType(attImg, types.OCIEmptyJSON)

	// Construct index containing both image and attestation
	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{
					OS:           "linux",
					Architecture: "amd64",
				},
			},
		},
		mutate.IndexAddendum{
			Add: attImg,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{
					OS:           "unknown",
					Architecture: "unknown",
				},
				Annotations: map[string]string{
					"vnd.docker.reference.type": "attestation-manifest",
				},
			},
		},
	)

	if err := validate.Index(idx); err != nil {
		t.Errorf("validate.Index(idx) = %v, want nil", err)
	}

	if err := validate.Index(idx, validate.Fast); err != nil {
		t.Errorf("validate.Index(idx, Fast) = %v, want nil", err)
	}
}

func TestIndex_PlatformVariant(t *testing.T) {
	img, err := random.Image(1024, 2)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	cf, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("img.ConfigFile: %v", err)
	}
	cf = cf.DeepCopy()
	cf.OS = "linux"
	cf.Architecture = "arm"
	cf.Variant = "v7"
	img, err = mutate.ConfigFile(img, cf)
	if err != nil {
		t.Fatalf("mutate.ConfigFile: %v", err)
	}

	// Matching variant
	idx := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
		Add: img,
		Descriptor: v1.Descriptor{
			Platform: &v1.Platform{
				OS:           "linux",
				Architecture: "arm",
				Variant:      "v7",
			},
		},
	})
	if err := validate.Index(idx); err != nil {
		t.Errorf("validate.Index(matching variant) = %v, want nil", err)
	}

	// Mismatched variant
	idxBad := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
		Add: img,
		Descriptor: v1.Descriptor{
			Platform: &v1.Platform{
				OS:           "linux",
				Architecture: "arm",
				Variant:      "v6",
			},
		},
	})
	err = validate.Index(idxBad)
	if err == nil {
		t.Fatal("validate.Index(idxBad) expected variant mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "mismatched Variant: v7 != v6") {
		t.Errorf("expected 'mismatched Variant: v7 != v6' in error, got %v", err)
	}
}
