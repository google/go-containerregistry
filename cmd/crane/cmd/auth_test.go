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
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/cli/cli/config"
)

func TestLoginRequiresBothUsernameAndPassword(t *testing.T) {
	tests := []struct {
		name string
		opts loginOptions
	}{
		{
			name: "username only",
			opts: loginOptions{serverAddress: "reg.example.com", user: "AzureDiamond"},
		},
		{
			name: "password only",
			opts: loginOptions{serverAddress: "reg.example.com", password: "hunter2"},
		},
		{
			name: "neither",
			opts: loginOptions{serverAddress: "reg.example.com"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOCKER_CONFIG", t.TempDir())
			err := login(tc.opts)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			cf, loadErr := config.Load(os.Getenv("DOCKER_CONFIG"))
			if loadErr != nil {
				t.Fatalf("failed to load docker config: %v", loadErr)
			}
			if _, ok := cf.AuthConfigs[tc.opts.serverAddress]; ok {
				t.Fatalf("expected no credentials to be stored for %q, but found some", tc.opts.serverAddress)
			}
		})
	}
}

func TestLoginStoresCredentialsWhenBothProvided(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", dir)

	opts := loginOptions{
		serverAddress: "reg.example.com",
		user:          "AzureDiamond",
		password:      "hunter2",
	}
	if err := login(opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatalf("expected config.json to be written: %v", err)
	}

	cf, err := config.Load(dir)
	if err != nil {
		t.Fatalf("failed to load docker config: %v", err)
	}
	stored, ok := cf.AuthConfigs[opts.serverAddress]
	if !ok {
		t.Fatalf("expected credentials to be stored for %q", opts.serverAddress)
	}
	if stored.Username != opts.user || stored.Password != opts.password {
		t.Fatalf("stored credentials %+v do not match input", stored)
	}
}

func TestLoginPasswordStdinEmptyStillRequiresPassword(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	opts := loginOptions{
		serverAddress: "reg.example.com",
		user:          "AzureDiamond",
		passwordStdin: true,
	}
	if err := login(opts); err == nil {
		t.Fatal("expected an error when stdin provides an empty password, got nil")
	}
}
