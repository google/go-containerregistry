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
	"io"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// TestIndexAppendAndFilterAnnotations checks that --annotation on index append
// (remote and local layout) and index filter sets index-level annotations, and
// that annotations already on the index survive.
func TestIndexAppendAndFilterAnnotations(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// The child's platform comes from its config, which is what index append
	// and index filter read when they resolve a -m reference.
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("ConfigFile: %v", err)
	}
	cfg = cfg.DeepCopy()
	cfg.OS, cfg.Architecture = "linux", "amd64"
	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		t.Fatalf("mutate.ConfigFile: %v", err)
	}
	imgRef := fmt.Sprintf("%s/test/index-child:latest", u.Host)
	if err := crane.Push(img, imgRef); err != nil {
		t.Fatalf("crane.Push: %v", err)
	}

	options := []crane.Option{}

	// Remote append creates the index with the annotation set.
	idxRef := fmt.Sprintf("%s/test/index-append:latest", u.Host)
	appendCmd := NewCmdIndexAppend(&options)
	appendCmd.SetArgs([]string{"-m", imgRef, "-t", idxRef, "-a", "org.opencontainers.image.title=my-idx"})
	appendCmd.SetOut(io.Discard)
	appendCmd.SetErr(io.Discard)
	if err := appendCmd.Execute(); err != nil {
		t.Fatalf("index append: %v", err)
	}
	idxName, err := name.ParseReference(idxRef)
	if err != nil {
		t.Fatalf("name.ParseReference: %v", err)
	}
	idx, err := remote.Index(idxName)
	if err != nil {
		t.Fatalf("remote.Index: %v", err)
	}
	m, err := idx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest: %v", err)
	}
	if got := m.Annotations["org.opencontainers.image.title"]; got != "my-idx" {
		t.Errorf("appended index title annotation = %q, want %q", got, "my-idx")
	}
	if len(m.Manifests) != 1 {
		t.Errorf("appended index has %d manifests, want 1", len(m.Manifests))
	}

	// Filter keeps the existing annotation and adds the new one.
	filteredRef := fmt.Sprintf("%s/test/index-filter:latest", u.Host)
	filterCmd := NewCmdIndexFilter(&options)
	filterCmd.SetArgs([]string{idxRef, "--platform", "linux/amd64", "-a", "filtered=true", "-t", filteredRef})
	filterCmd.SetOut(io.Discard)
	filterCmd.SetErr(io.Discard)
	if err := filterCmd.Execute(); err != nil {
		t.Fatalf("index filter: %v", err)
	}
	filteredName, err := name.ParseReference(filteredRef)
	if err != nil {
		t.Fatalf("name.ParseReference: %v", err)
	}
	filtered, err := remote.Index(filteredName)
	if err != nil {
		t.Fatalf("remote.Index: %v", err)
	}
	fm, err := filtered.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest: %v", err)
	}
	if got := fm.Annotations["org.opencontainers.image.title"]; got != "my-idx" {
		t.Errorf("filtered index title annotation = %q, want %q", got, "my-idx")
	}
	if got := fm.Annotations["filtered"]; got != "true" {
		t.Errorf("filtered index annotation filtered = %q, want %q", got, "true")
	}

	// Local append creates an OCI layout whose index.json carries the annotation.
	layoutDir := filepath.Join(t.TempDir(), "layout")
	localCmd := NewCmdIndexAppend(&options)
	localCmd.SetArgs([]string{layoutDir, "-m", imgRef, "-a", "local=ann"})
	localCmd.SetOut(io.Discard)
	localCmd.SetErr(io.Discard)
	if err := localCmd.Execute(); err != nil {
		t.Fatalf("local index append: %v", err)
	}
	localIdx, err := layout.ImageIndexFromPath(layoutDir)
	if err != nil {
		t.Fatalf("layout.ImageIndexFromPath: %v", err)
	}
	lm, err := localIdx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest: %v", err)
	}
	if got := lm.Annotations["local"]; got != "ann" {
		t.Errorf("local layout annotation local = %q, want %q", got, "ann")
	}
	if len(lm.Manifests) != 1 {
		t.Errorf("local layout has %d manifests, want 1", len(lm.Manifests))
	}
}
