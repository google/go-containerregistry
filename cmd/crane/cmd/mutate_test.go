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
	"io"
	"net/http/httptest"
	"net/url"
	"path"
	"slices"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// TestMutateEntrypointCmdPreservation verifies that --entrypoint and --cmd are
// independently respected: overriding one preserves the other inherited from
// the base image. Regression test for #2041.
func TestMutateEntrypointCmdPreservation(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	repo := path.Join(u.Host, "test/mutate")

	// Build a base image with both Entrypoint and Cmd populated.
	base, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := base.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	cfg = cfg.DeepCopy()
	cfg.Config.Entrypoint = []string{"/old-entry"}
	cfg.Config.Cmd = []string{"old-cmd-arg"}
	base, err = mutate.ConfigFile(base, cfg)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		args         []string
		wantEntry    []string
		wantCmd      []string
	}{
		{
			name:      "only --entrypoint preserves base Cmd",
			args:      []string{"--entrypoint", "/new-entry"},
			wantEntry: []string{"/new-entry"},
			wantCmd:   []string{"old-cmd-arg"},
		},
		{
			name:      "only --cmd preserves base Entrypoint",
			args:      []string{"--cmd", "new-cmd-arg"},
			wantEntry: []string{"/old-entry"},
			wantCmd:   []string{"new-cmd-arg"},
		},
		{
			name:      "both flags override both",
			args:      []string{"--entrypoint", "/new-entry", "--cmd", "new-cmd-arg"},
			wantEntry: []string{"/new-entry"},
			wantCmd:   []string{"new-cmd-arg"},
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srcRef, err := name.ParseReference(repo + ":src")
			if err != nil {
				t.Fatal(err)
			}
			if err := remote.Write(srcRef, base); err != nil {
				t.Fatalf("seeding src: %v", err)
			}

			// Each subtest writes to a distinct mutated tag.
			dstTag := repo + ":mutated-" + tc.name[:4] + string(rune('a'+i))

			options := []crane.Option{}
			c := NewCmdMutate(&options)
			c.SetOut(io.Discard)
			c.SetErr(io.Discard)
			c.SetArgs(append([]string{srcRef.String(), "--tag", dstTag}, tc.args...))
			if err := c.Execute(); err != nil {
				t.Fatalf("mutate: %v", err)
			}

			mutatedRef, err := name.ParseReference(dstTag)
			if err != nil {
				t.Fatal(err)
			}
			mutated, err := remote.Image(mutatedRef)
			if err != nil {
				t.Fatalf("fetching mutated: %v", err)
			}
			mcfg, err := mutated.ConfigFile()
			if err != nil {
				t.Fatalf("ConfigFile: %v", err)
			}
			if !slices.Equal(mcfg.Config.Entrypoint, tc.wantEntry) {
				t.Errorf("Entrypoint = %v, want %v", mcfg.Config.Entrypoint, tc.wantEntry)
			}
			if !slices.Equal(mcfg.Config.Cmd, tc.wantCmd) {
				t.Errorf("Cmd = %v, want %v", mcfg.Config.Cmd, tc.wantCmd)
			}
		})
	}
}
