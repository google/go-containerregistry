// Copyright 2026 The original author or authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package layout_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
)

// Compile these from an external package to verify unchanged method-expression
// types, not just that existing direct calls can still compile.
var _ func(layout.Path, v1.Hash) (io.ReadCloser, error) = layout.Path.Blob
var _ func(layout.Path, v1.Hash) ([]byte, error) = layout.Path.Bytes
var _ func(layout.Path, v1.Hash) (v1.Image, error) = layout.Path.Image
var _ func(layout.Path) (v1.ImageIndex, error) = layout.Path.ImageIndex
var _ func(string) (v1.ImageIndex, error) = layout.ImageIndexFromPath

func TestExternalReaderContract(t *testing.T) {
	l := layout.Path(t.TempDir())
	h := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("e", 64)}
	blob := filepath.Join(string(l), "blobs", h.Algorithm, h.Hex)
	if err := os.MkdirAll(filepath.Dir(blob), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "shared-layer")
	if err := os.WriteFile(target, []byte("shared local bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, blob); err != nil {
		t.Fatal(err)
	}
	trusted := l.Reader(layout.WithAllowFinalSymlinks())
	if _, err := l.Bytes(h); err == nil {
		t.Fatal("legacy Path read accepted symlink")
	}
	if _, err := l.Reader().Bytes(h); err == nil {
		t.Fatal("default Reader accepted symlink")
	}
	got, err := trusted.Bytes(h)
	if err != nil || string(got) != "shared local bytes" {
		t.Fatalf("trusted read: %q, %v", got, err)
	}
	if _, err := l.Bytes(h); err == nil {
		t.Fatal("trusted read changed legacy Path policy")
	}
	t.Log("positive: explicit trusted reader returned shared local bytes")
	t.Log("negative: legacy Path and default Reader rejected the same symlink before and after opt-in read")
}
