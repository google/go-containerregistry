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
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
)

func TestMutateAllowsEmptyLabelValue(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	ref := fmt.Sprintf("%s/test/mutate:latest", u.Host)
	options := []crane.Option{crane.Insecure}
	if err := crane.Push(empty.Image, ref, options...); err != nil {
		t.Fatalf("crane.Push: %v", err)
	}

	cmd := NewCmdMutate(&options)
	if err := cmd.Flags().Set("label", "my.key="); err != nil {
		t.Fatalf("setting label: %v", err)
	}
	if err := cmd.RunE(cmd, []string{ref}); err != nil {
		t.Fatalf("NewCmdMutate: %v", err)
	}

	img, err := crane.Pull(ref, options...)
	if err != nil {
		t.Fatalf("crane.Pull: %v", err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("ConfigFile: %v", err)
	}
	if got, ok := cfg.Config.Labels["my.key"]; !ok || got != "" {
		t.Errorf("label my.key = %q, %t; want empty value present", got, ok)
	}
}
