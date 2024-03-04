// Copyright 2019 The original author or authors
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

package layout

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
)

var (
	testRefPath = filepath.Join("testdata", "test_with_ref")
)

func TestPullerHeadWithDigest(t *testing.T) {
	path, err := FromPath(testRefPath)
	if err != nil {
		t.Fatalf("FromPath() = %v", err)
	}
	digest := "sha256:32589985702551b6c56033bb3334432a0a513bf9d6aceda0f67c42b003850720"
	puller := NewPuller(path)
	desc, err := puller.Head(context.TODO(), name.MustParseReference("reg.local/repo2@sha256:32589985702551b6c56033bb3334432a0a513bf9d6aceda0f67c42b003850720"))
	if err != nil {
		t.Fatalf("puller.Head() = %v", err)
	}

	if desc.Digest.String() != digest {
		t.Fatalf("wrong descriptor returned, expected %s but got %s ", digest, desc.Digest)
	}
}

func TestPullerHeadWithTag(t *testing.T) {
	path, err := FromPath(testRefPath)
	if err != nil {
		t.Fatalf("FromPath() = %v", err)
	}
	digest := "sha256:05f95b26ed10668b7183c1e2da98610e91372fa9f510046d4ce5812addad86b5"
	puller := NewPuller(path)
	desc, err := puller.Head(context.TODO(), name.MustParseReference("reg.local/repo4:latest"))
	if err != nil {
		t.Fatalf("puller.Head() = %v", err)
	}
	if desc.Digest.String() != digest {
		t.Fatalf("wrong descriptor returned, expected %s but got %s ", digest, desc.Digest)
	}
}

func TestPullerArtifact(t *testing.T) {
	path, err := FromPath(testRefPath)
	if err != nil {
		t.Fatalf("FromPath() = %v", err)
	}
	expectedDigest := "sha256:05f95b26ed10668b7183c1e2da98610e91372fa9f510046d4ce5812addad86b5"

	puller := NewPuller(path)
	desc, err := puller.Artifact(context.TODO(), name.MustParseReference("reg.local/repo4:latest"))
	if err != nil {
		t.Fatalf("puller.Artifact() = %v", err)
	}

	digest, err := desc.Digest()
	if err != nil {
		t.Fatalf("desc.Digest() = %v", err)
	}

	mt, err := desc.MediaType()
	if err != nil {
		t.Fatalf("desc.MediaType() = %v", err)
	}

	if digest.String() != expectedDigest {
		t.Fatalf("wrong descriptor returned, expected %s but got %s ", expectedDigest, digest.String())
	}

	if !mt.IsIndex() {
		t.Fatalf("expcted an image index but got %s", mt)
	}
}

func TestPullerLayer(t *testing.T) {
	path, err := FromPath(testRefPath)
	if err != nil {
		t.Fatalf("FromPath() = %v", err)
	}

	expectedDigest := "sha256:6e0b05049ed9c17d02e1a55e80d6599dbfcce7f4f4b022e3c673e685789c470e"
	puller := NewPuller(path)

	layer, err := puller.Layer(context.TODO(), name.MustParseReference("reg.local/repo4@sha256:6e0b05049ed9c17d02e1a55e80d6599dbfcce7f4f4b022e3c673e685789c470e").(name.Digest))
	if err != nil {
		t.Fatalf("puller.Layer() = %v", err)
	}

	digest, err := layer.Digest()
	if err != nil {
		t.Fatalf("layer.Digest() = %v", err)
	}

	if digest.String() != expectedDigest {
		t.Fatalf("wrong descriptor returned, expected %s but got %s ", expectedDigest, digest.String())
	}

	size, err := layer.Size()
	if err != nil {
		t.Fatalf("layer.Size() = %v", err)
	}

	if size != 4096 {
		t.Fatalf("wrong size returned, expected 320 but got %d", size)
	}
}
