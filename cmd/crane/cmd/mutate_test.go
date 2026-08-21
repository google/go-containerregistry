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
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

// An empty annotation value is explicitly allowed by the image spec, and an
// empty label value is the config-level equivalent.
func TestMutateEmptyLabelAndAnnotationValues(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	src := fmt.Sprintf("%s/test/mutate:latest", u.Host)

	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	if err := crane.Push(img, src); err != nil {
		t.Fatalf("crane.Push: %v", err)
	}

	options := []crane.Option{}
	cmd := NewCmdMutate(&options)
	cmd.SetArgs([]string{src, "--label", "empty.label=", "--annotation", "empty.annotation="})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mutate with empty values: %v", err)
	}

	pulled, err := crane.Pull(src)
	if err != nil {
		t.Fatalf("crane.Pull: %v", err)
	}

	cfg, err := pulled.ConfigFile()
	if err != nil {
		t.Fatalf("ConfigFile: %v", err)
	}
	got, ok := cfg.Config.Labels["empty.label"]
	if !ok {
		t.Errorf("label %q was not set at all", "empty.label")
	} else if got != "" {
		t.Errorf("label %q = %q, want empty string", "empty.label", got)
	}

	manifest, err := pulled.Manifest()
	if err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	got, ok = manifest.Annotations["empty.annotation"]
	if !ok {
		t.Errorf("annotation %q was not set at all", "empty.annotation")
	} else if got != "" {
		t.Errorf("annotation %q = %q, want empty string", "empty.annotation", got)
	}
}
