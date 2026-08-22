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

package validate

import (
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

// attestation mimics a BuildKit attestation manifest: a manifest whose
// config is not an image config and whose layers are not image layers.
func attestation(t *testing.T) v1.Image {
	t.Helper()
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	return mutate.ConfigMediaType(img, "application/vnd.oci.empty.v1+json")
}

func attestationAddendum(img v1.Image) mutate.IndexAddendum {
	return mutate.IndexAddendum{
		Add: img,
		Descriptor: v1.Descriptor{
			Annotations: map[string]string{
				"vnd.docker.reference.type": "attestation-manifest",
			},
			Platform: &v1.Platform{
				Architecture: "unknown",
				OS:           "unknown",
			},
		},
	}
}

func TestIndexAcceptsArtifactManifests(t *testing.T) {
	base, err := random.Index(1024, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	idx := mutate.AppendManifests(base, attestationAddendum(attestation(t)))
	if err := Index(idx); err != nil {
		t.Errorf("Index() = %v, want nil", err)
	}
	if err := Index(idx, Fast); err != nil {
		t.Errorf("Index(Fast) = %v, want nil", err)
	}
}

func TestIndexAcceptsArtifactManifestsByDescriptorArtifactType(t *testing.T) {
	base, err := random.Index(1024, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	// An artifact that keeps an image config media type but declares an
	// artifactType on its index descriptor; its config would fail image
	// invariants.
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	img, err = mutate.ConfigFile(img, &v1.ConfigFile{})
	if err != nil {
		t.Fatal(err)
	}

	idx := mutate.AppendManifests(base, mutate.IndexAddendum{
		Add: img,
		Descriptor: v1.Descriptor{
			ArtifactType: "application/vnd.example.artifact",
		},
	})
	if err := Index(idx); err != nil {
		t.Errorf("Index() = %v, want nil", err)
	}
}

type tamperedConfig struct {
	v1.Image
}

func (i tamperedConfig) RawConfigFile() ([]byte, error) {
	return []byte(`{"tampered":true}`), nil
}

func TestIndexRejectsCorruptArtifactManifests(t *testing.T) {
	base, err := random.Index(1024, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	idx := mutate.AppendManifests(base, attestationAddendum(tamperedConfig{attestation(t)}))
	err = Index(idx)
	if err == nil {
		t.Fatal("Index() = nil, want error for artifact with corrupt config")
	}
	if !strings.Contains(err.Error(), "mismatched config digest") {
		t.Errorf("Index() = %v, want mismatched config digest", err)
	}
}

func TestImageErrorsOnMissingDiffIDs(t *testing.T) {
	img, err := random.Image(1024, 2)
	if err != nil {
		t.Fatal(err)
	}
	img, err = mutate.ConfigFile(img, &v1.ConfigFile{})
	if err != nil {
		t.Fatal(err)
	}

	err = Image(img)
	if err == nil {
		t.Fatal("Image() = nil, want error for config without diff_ids")
	}
	if !strings.Contains(err.Error(), "missing layer[0] diffid") {
		t.Errorf("Image() = %v, want missing layer[0] diffid", err)
	}
}
