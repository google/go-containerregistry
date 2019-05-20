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

package partial_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

// Remote leverages a lot of compressed partials.
func TestRemote(t *testing.T) {
	rnd, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	s, err := registry.TLS("gcr.io")
	if err != nil {
		t.Fatal(err)
	}
	tr := s.Client().Transport

	src := "gcr.io/test/compressed"
	ref, err := name.ParseReference(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, rnd, remote.WithTransport(tr)); err != nil {
		t.Fatal(err)
	}

	img, err := remote.Image(ref, remote.WithTransport(tr))
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Image(img); err != nil {
		t.Fatal(err)
	}

	cf, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	m, err := img.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	layer, err := img.LayerByDiffID(cf.RootFS.DiffIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	d, err := layer.Digest()
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(d, m.Layers[0].Digest); diff != "" {
		t.Errorf("mismatched digest: %v", diff)
	}
}
