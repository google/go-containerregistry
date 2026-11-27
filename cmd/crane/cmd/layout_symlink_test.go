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
	"bytes"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
)

type symlinkedImageLayout struct {
	path          string
	imageDigest   v1.Hash
	imageManifest []byte
	indexDigest   v1.Hash
	indexManifest []byte
}

func TestPushAllowLayoutSymlinksImage(t *testing.T) {
	l := newSymlinkedImageLayout(t)
	host := newOwnedRegistry(t)

	strictTag := fmt.Sprintf("%s/test/push-image:strict", host)
	if _, err := executePush(t, l.path, strictTag); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("strict push error = %v, want final-symlink rejection", err)
	}

	trustedTag := fmt.Sprintf("%s/test/push-image:trusted", host)
	out, err := executePush(t, "--allow-layout-symlinks", l.path, trustedTag)
	if err != nil {
		t.Fatalf("trusted push = %v", err)
	}
	if !strings.Contains(out, l.imageDigest.String()) {
		t.Fatalf("trusted push output = %q, want digest %s", out, l.imageDigest)
	}

	got := readRemoteImage(t, trustedTag)
	gotDigest, err := got.Digest()
	if err != nil {
		t.Fatalf("remote image Digest() = %v", err)
	}
	if gotDigest != l.imageDigest {
		t.Fatalf("remote image digest = %s, want %s", gotDigest, l.imageDigest)
	}
	gotManifest, err := got.RawManifest()
	if err != nil {
		t.Fatalf("remote image RawManifest() = %v", err)
	}
	if !bytes.Equal(gotManifest, l.imageManifest) {
		t.Fatal("remote image manifest differs from the symlink target bytes")
	}

	strictAfterTag := fmt.Sprintf("%s/test/push-image:strict-after", host)
	if _, err := executePush(t, l.path, strictAfterTag); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("strict push after opted-in call error = %v, want final-symlink rejection", err)
	}
}

func TestPushAllowLayoutSymlinksIndex(t *testing.T) {
	l := newSymlinkedImageLayout(t)
	host := newOwnedRegistry(t)

	strictTag := fmt.Sprintf("%s/test/push-index:strict", host)
	if _, err := executePush(t, "--index", l.path, strictTag); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("strict index push error = %v, want final-symlink rejection", err)
	}

	trustedTag := fmt.Sprintf("%s/test/push-index:trusted", host)
	out, err := executePush(t, "--index", "--allow-layout-symlinks", l.path, trustedTag)
	if err != nil {
		t.Fatalf("trusted index push = %v", err)
	}
	if !strings.Contains(out, l.indexDigest.String()) {
		t.Fatalf("trusted index push output = %q, want digest %s", out, l.indexDigest)
	}

	got := readRemoteIndex(t, trustedTag)
	gotDigest, err := got.Digest()
	if err != nil {
		t.Fatalf("remote index Digest() = %v", err)
	}
	if gotDigest != l.indexDigest {
		t.Fatalf("remote index digest = %s, want %s", gotDigest, l.indexDigest)
	}
	gotManifest, err := got.RawManifest()
	if err != nil {
		t.Fatalf("remote index RawManifest() = %v", err)
	}
	if !bytes.Equal(gotManifest, l.indexManifest) {
		t.Fatal("remote index manifest differs from the local index.json bytes")
	}
	child, err := got.Image(l.imageDigest)
	if err != nil {
		t.Fatalf("remote index Image(%s) = %v", l.imageDigest, err)
	}
	childManifest, err := child.RawManifest()
	if err != nil {
		t.Fatalf("remote child RawManifest() = %v", err)
	}
	if !bytes.Equal(childManifest, l.imageManifest) {
		t.Fatal("remote child manifest differs from the symlink target bytes")
	}
}

func TestIndexAppendAllowLayoutSymlinksManifest(t *testing.T) {
	l := newSymlinkedImageLayout(t)
	host := newOwnedRegistry(t)

	strictTag := fmt.Sprintf("%s/test/index-append:strict", host)
	if _, err := executeIndexAppend(t, "--manifest", l.path, "--tag", strictTag); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("strict index append error = %v, want final-symlink rejection", err)
	}

	trustedTag := fmt.Sprintf("%s/test/index-append:trusted", host)
	out, err := executeIndexAppend(t, "--manifest", l.path, "--tag", trustedTag, "--allow-layout-symlinks")
	if err != nil {
		t.Fatalf("trusted index append = %v", err)
	}

	got := readRemoteIndex(t, trustedTag)
	gotDigest, err := got.Digest()
	if err != nil {
		t.Fatalf("remote appended index Digest() = %v", err)
	}
	if !strings.Contains(out, gotDigest.String()) {
		t.Fatalf("trusted index append output = %q, want digest %s", out, gotDigest)
	}
	im, err := got.IndexManifest()
	if err != nil {
		t.Fatalf("remote appended IndexManifest() = %v", err)
	}
	if len(im.Manifests) != 1 {
		t.Fatalf("remote appended index contains %d manifests, want 1", len(im.Manifests))
	}
	if im.Manifests[0].Digest != l.imageDigest {
		t.Fatalf("remote appended child digest = %s, want %s", im.Manifests[0].Digest, l.imageDigest)
	}
	child, err := got.Image(l.imageDigest)
	if err != nil {
		t.Fatalf("remote appended index Image(%s) = %v", l.imageDigest, err)
	}
	childManifest, err := child.RawManifest()
	if err != nil {
		t.Fatalf("remote appended child RawManifest() = %v", err)
	}
	if !bytes.Equal(childManifest, l.imageManifest) {
		t.Fatal("remote appended child manifest differs from the symlink target bytes")
	}
}

func TestLayoutGCAllowLayoutSymlinks(t *testing.T) {
	l := newSymlinkedImageLayout(t)

	if _, err := executeCobra(t, newCmdGc(), l.path); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("strict layout gc error = %v, want final-symlink rejection", err)
	}
	if _, err := executeCobra(t, newCmdGc(), "--allow-layout-symlinks", l.path); err != nil {
		t.Fatalf("trusted layout gc = %v", err)
	}
	if _, err := executeCobra(t, newCmdGc(), l.path); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("strict layout gc after opted-in call error = %v, want final-symlink rejection", err)
	}
}

func newSymlinkedImageLayout(t *testing.T) symlinkedImageLayout {
	t.Helper()
	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("random.Image() = %v", err)
	}
	p, err := layout.Write(filepath.Join(t.TempDir(), "layout"), empty.Index)
	if err != nil {
		t.Fatalf("layout.Write() = %v", err)
	}
	if err := p.AppendImage(img); err != nil {
		t.Fatalf("AppendImage() = %v", err)
	}

	imageDigest, err := img.Digest()
	if err != nil {
		t.Fatalf("image Digest() = %v", err)
	}
	imageManifest, err := img.RawManifest()
	if err != nil {
		t.Fatalf("image RawManifest() = %v", err)
	}
	idx, err := p.ImageIndex()
	if err != nil {
		t.Fatalf("Path.ImageIndex() = %v", err)
	}
	indexDigest, err := idx.Digest()
	if err != nil {
		t.Fatalf("index Digest() = %v", err)
	}
	indexManifest, err := idx.RawManifest()
	if err != nil {
		t.Fatalf("index RawManifest() = %v", err)
	}

	blob := filepath.Join(string(p), "blobs", imageDigest.Algorithm, imageDigest.Hex)
	target := filepath.Join(t.TempDir(), "image-manifest")
	if err := os.WriteFile(target, imageManifest, 0o600); err != nil {
		t.Fatalf("WriteFile(symlink target) = %v", err)
	}
	if err := os.Remove(blob); err != nil {
		t.Fatalf("Remove(image manifest blob) = %v", err)
	}
	if err := os.Symlink(target, blob); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks are unavailable in this Windows test environment: %v", err)
		}
		t.Fatalf("Symlink(image manifest blob) = %v", err)
	}

	return symlinkedImageLayout{
		path:          string(p),
		imageDigest:   imageDigest,
		imageManifest: imageManifest,
		indexDigest:   indexDigest,
		indexManifest: indexManifest,
	}
}

func newOwnedRegistry(t *testing.T) string {
	t.Helper()
	s := httptest.NewServer(registry.New())
	t.Cleanup(s.Close)
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatalf("url.Parse(%q) = %v", s.URL, err)
	}
	return u.Host
}

func executePush(t *testing.T, args ...string) (string, error) {
	t.Helper()
	options := []crane.Option{crane.Insecure}
	return executeCobra(t, NewCmdPush(&options), args...)
}

func executeIndexAppend(t *testing.T, args ...string) (string, error) {
	t.Helper()
	options := []crane.Option{crane.Insecure}
	return executeCobra(t, NewCmdIndexAppend(&options), args...)
}

func executeCobra(t *testing.T, command *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs(args)
	err := command.Execute()
	return stdout.String(), err
}

func readRemoteImage(t *testing.T, raw string) v1.Image {
	t.Helper()
	ref := insecureReference(t, raw)
	img, err := remote.Image(ref)
	if err != nil {
		t.Fatalf("remote.Image(%s) = %v", raw, err)
	}
	return img
}

func readRemoteIndex(t *testing.T, raw string) v1.ImageIndex {
	t.Helper()
	ref := insecureReference(t, raw)
	idx, err := remote.Index(ref)
	if err != nil {
		t.Fatalf("remote.Index(%s) = %v", raw, err)
	}
	return idx
}

func insecureReference(t *testing.T, raw string) name.Reference {
	t.Helper()
	ref, err := name.ParseReference(raw, name.Insecure)
	if err != nil {
		t.Fatalf("name.ParseReference(%q) = %v", raw, err)
	}
	return ref
}
