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

package cmd

import (
	"bytes"
	"io"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

func mustRegistry(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	s := httptest.NewServer(registry.New())
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	return s, u.Host
}

func TestEditConfig(t *testing.T) {
	reg, host := mustRegistry(t)
	defer reg.Close()
	src := path.Join(host, "crane/edit/config")

	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := crane.Push(img, src); err != nil {
		t.Fatal(err)
	}

	cmd := NewCmdEditConfig(&[]crane.Option{})
	cmd.SetArgs([]string{src})
	cmd.SetIn(strings.NewReader("{}"))

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestEditManifest(t *testing.T) {
	reg, host := mustRegistry(t)
	defer reg.Close()
	src := path.Join(host, "crane/edit/manifest")

	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := crane.Push(img, src); err != nil {
		t.Fatal(err)
	}

	cmd := NewCmdEditManifest(&[]crane.Option{})
	cmd.SetArgs([]string{src})
	cmd.SetIn(strings.NewReader("{}"))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestEditFilesystem(t *testing.T) {
	reg, host := mustRegistry(t)
	defer reg.Close()
	src := path.Join(host, "crane/edit/fs")

	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := crane.Push(img, src); err != nil {
		t.Fatal(err)
	}

	cmd := NewCmdEditFs(&[]crane.Option{})
	cmd.SetArgs([]string{src})
	cmd.Flags().Set("filename", "/foo/bar")
	cmd.SetIn(strings.NewReader("baz"))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	img, err = crane.Pull(src)
	if err != nil {
		t.Fatal(err)
	}

	r, _, err := findFile(img, "/foo/bar")
	if err != nil {
		t.Fatal(err)
	}

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, []byte("baz")) {
		t.Fatalf("got: %s, want %s", got, "baz")
	}

	// Edit the same file to make sure we can edit existing files.
	cmd = NewCmdEditFs(&[]crane.Option{})
	cmd.SetArgs([]string{src})
	cmd.Flags().Set("filename", "/foo/bar")
	cmd.SetIn(strings.NewReader("quux"))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	img, err = crane.Pull(src)
	if err != nil {
		t.Fatal(err)
	}

	r, _, err = findFile(img, "/foo/bar")
	if err != nil {
		t.Fatal(err)
	}

	got, err = io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, []byte("quux")) {
		t.Fatalf("got: %s, want %s", got, "quux")
	}
}

func TestFindFile(t *testing.T) {
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	r, h, err := findFile(img, "/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}

	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 0 {
		t.Errorf("expected empty reader, got: %s", string(b))
	}

	if h.Name != "/does-not-exist" {
		t.Errorf("tar.Header has wrong name: %v", h)
	}
}
