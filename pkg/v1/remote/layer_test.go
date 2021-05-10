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

package remote

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/internal/compare"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestRemoteLayer(t *testing.T) {
	layer, err := random.Layer(1024, types.DockerLayer)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := layer.Digest()
	if err != nil {
		t.Fatal(err)
	}

	// Set up a fake registry and write what we pulled to it.
	// This ensures we get coverage for the remoteLayer.MediaType path.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	t.Log(s.URL)
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(u)
	dst := fmt.Sprintf("%s/some/path@%s", u.Host, digest)
	t.Log(dst)
	ref, err := name.NewDigest(dst)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(ref)
	if err := WriteLayer(ref.Context(), layer); err != nil {
		t.Fatalf("failed to WriteLayer: %v", err)
	}

	got, err := Layer(ref)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := got.MediaType(); err != nil {
		t.Errorf("reading MediaType: %v", err)
	}

	if err := compare.Layers(got, layer); err != nil {
		t.Errorf("compare.Layers: %v", err)
	}
	if err := validate.Layer(got); err != nil {
		t.Errorf("validate.Layer: %v", err)
	}

	if ok, err := partial.Exists(got); err != nil {
		t.Fatal(err)
	} else if got, want := ok, true; got != want {
		t.Errorf("Exists() = %t != %t", got, want)
	}
}

func TestRemoteLayerDescriptor(t *testing.T) {
	layer, err := random.Layer(1024, types.DockerLayer)
	if err != nil {
		t.Fatal(err)
	}
	image, err := mutate.Append(empty.Image, mutate.Addendum{
		Layer: layer,
		URLs:  []string{"example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set up a fake registry and write what we pulled to it.
	// This ensures we get coverage for the remoteLayer.MediaType path.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	dst := fmt.Sprintf("%s/some/path:tag", u.Host)
	ref, err := name.ParseReference(dst)
	if err != nil {
		t.Fatal(err)
	}

	if err := Write(ref, image); err != nil {
		t.Fatalf("failed to WriteLayer: %v", err)
	}

	pulled, err := Image(ref)
	if err != nil {
		t.Fatal(err)
	}

	layers, err := pulled.Layers()
	if err != nil {
		t.Fatal(err)
	}

	desc, err := partial.Descriptor(layers[0])
	if err != nil {
		t.Fatal(err)
	}

	if len(desc.URLs) != 1 {
		t.Fatalf("expected url for layer[0]")
	}

	if got, want := desc.URLs[0], "example.com"; got != want {
		t.Errorf("layer[0].urls[0] = %s != %s", got, want)
	}
	if ok, err := partial.Exists(layers[0]); err != nil {
		t.Fatal(err)
	} else if got, want := ok, true; got != want {
		t.Errorf("Exists() = %t != %t", got, want)
	}
}
