// Copyright 2022 Google LLC All Rights Reserved.
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
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestAppendWithOCIBaseImage(t *testing.T) {
	base := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	img, err := crane.Append(base, "testdata/content.tar")

	if err != nil {
		t.Fatalf("crane.Append(): %v", err)
	}

	layers, err := img.Layers()

	if err != nil {
		t.Fatalf("img.Layers(): %v", err)
	}

	mediaType, err := layers[0].MediaType()

	if err != nil {
		t.Fatalf("layers[0].MediaType(): %v", err)
	}

	if got, want := mediaType, types.OCILayer; got != want {
		t.Errorf("MediaType(): want %q, got %q", want, got)
	}
}

func TestAppendWithDockerBaseImage(t *testing.T) {
	img, err := crane.Append(empty.Image, "testdata/content.tar")

	if err != nil {
		t.Fatalf("crane.Append(): %v", err)
	}

	layers, err := img.Layers()

	if err != nil {
		t.Fatalf("img.Layers(): %v", err)
	}

	mediaType, err := layers[0].MediaType()

	if err != nil {
		t.Fatalf("layers[0].MediaType(): %v", err)
	}

	if got, want := mediaType, types.DockerLayer; got != want {
		t.Errorf("MediaType(): want %q, got %q", want, got)
	}
}
