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

package layout

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

var readerTestLayerDigest = v1.Hash{
	Algorithm: "sha256",
	Hex:       "dc52c6e48a1d51a96047b059f16889bc889c4b4c28f3b36b3f93187f62fc0b2b",
}

func TestReaderFinalSymlinkPolicy(t *testing.T) {
	l, h := newFinalSymlinkBlob(t, []byte("trusted symlink contents"))

	if _, err := l.Bytes(h); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("Path.Bytes() error = %v, want symlink rejection", err)
	}

	strict := l.Reader()
	if _, err := strict.Bytes(h); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("strict Reader.Bytes() error = %v, want symlink rejection", err)
	}

	trusted := l.Reader(WithAllowFinalSymlinks())
	want := []byte("trusted symlink contents")
	got, err := trusted.Bytes(h)
	if err != nil {
		t.Fatalf("trusted Reader.Bytes() = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("trusted Reader.Bytes() = %q, want %q", got, want)
	}

	rc, err := trusted.Blob(h)
	if err != nil {
		t.Fatalf("trusted Reader.Blob() = %v", err)
	}
	got, readErr := io.ReadAll(rc)
	closeErr := rc.Close()
	if readErr != nil {
		t.Fatalf("ReadAll(Reader.Blob()) = %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("Reader.Blob().Close() = %v", closeErr)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("trusted Reader.Blob() read %q, want %q", got, want)
	}

	regularHash := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("d", 64)}
	regularContents := []byte("regular contents")
	if err := os.WriteFile(l.blobPath(regularHash), regularContents, 0o644); err != nil {
		t.Fatalf("WriteFile(regular blob) = %v", err)
	}
	for name, reader := range map[string]Reader{"strict": strict, "trusted": trusted} {
		got, err := reader.Bytes(regularHash)
		if err != nil {
			t.Fatalf("%s Reader.Bytes(regular blob) = %v", name, err)
		}
		if !bytes.Equal(got, regularContents) {
			t.Fatalf("%s Reader.Bytes(regular blob) = %q, want %q", name, got, regularContents)
		}
	}

	if _, err := l.Bytes(h); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("Path.Bytes() after trusted read error = %v, want unchanged strict behavior", err)
	}
}

func TestReaderRejectsInvalidAndNonRegularTargets(t *testing.T) {
	h := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("e", 64)}

	t.Run("directory", func(t *testing.T) {
		l := Path(t.TempDir())
		if err := os.MkdirAll(l.blobPath(h), 0o755); err != nil {
			t.Fatalf("MkdirAll(blob directory) = %v", err)
		}
		for name, reader := range map[string]Reader{
			"strict":  l.Reader(),
			"trusted": l.Reader(WithAllowFinalSymlinks()),
		} {
			if _, err := reader.Bytes(h); err == nil || !strings.Contains(err.Error(), "not a regular file") {
				t.Fatalf("%s Reader.Bytes(directory) error = %v, want non-regular rejection", name, err)
			}
		}
	})

	t.Run("symlink to directory", func(t *testing.T) {
		l := Path(t.TempDir())
		if err := os.MkdirAll(filepath.Dir(l.blobPath(h)), 0o755); err != nil {
			t.Fatalf("MkdirAll(blobs) = %v", err)
		}
		target := t.TempDir()
		if err := os.Symlink(target, l.blobPath(h)); err != nil {
			t.Skipf("Symlink() = %v", err)
		}
		if _, err := l.Reader(WithAllowFinalSymlinks()).Bytes(h); err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("trusted Reader.Bytes(symlink to directory) error = %v, want non-regular rejection", err)
		}
	})

	t.Run("dangling symlink", func(t *testing.T) {
		l := Path(t.TempDir())
		if err := os.MkdirAll(filepath.Dir(l.blobPath(h)), 0o755); err != nil {
			t.Fatalf("MkdirAll(blobs) = %v", err)
		}
		target := filepath.Join(t.TempDir(), "missing")
		if err := os.Symlink(target, l.blobPath(h)); err != nil {
			t.Skipf("Symlink() = %v", err)
		}
		if _, err := l.Reader(WithAllowFinalSymlinks()).Bytes(h); err == nil {
			t.Fatal("trusted Reader.Bytes(dangling symlink) = nil, want error")
		}
	})
}

func TestReaderPolicyPropagation(t *testing.T) {
	t.Run("nested index", func(t *testing.T) {
		l := cloneLayoutFixture(t)
		want := replaceLayoutBlobWithSymlink(t, l, indexDigest)

		strictIndex, err := l.ImageIndex()
		if err != nil {
			t.Fatalf("Path.ImageIndex() = %v", err)
		}
		if _, err := strictIndex.ImageIndex(indexDigest); err == nil {
			t.Fatal("strict ImageIndex().ImageIndex() opened symlink blob")
		}

		trustedIndex, err := l.Reader(WithAllowFinalSymlinks()).ImageIndex()
		if err != nil {
			t.Fatalf("trusted Reader.ImageIndex() = %v", err)
		}
		nested, err := trustedIndex.ImageIndex(indexDigest)
		if err != nil {
			t.Fatalf("trusted ImageIndex().ImageIndex() = %v", err)
		}
		got, err := nested.RawManifest()
		if err != nil {
			t.Fatalf("nested.RawManifest() = %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("nested.RawManifest() differs from symlink target")
		}
	})

	t.Run("image manifest", func(t *testing.T) {
		l := cloneLayoutFixture(t)
		want := replaceLayoutBlobWithSymlink(t, l, manifestDigest)

		strictImage, err := l.Image(manifestDigest)
		if err != nil {
			t.Fatalf("Path.Image() = %v", err)
		}
		if _, err := strictImage.RawManifest(); err == nil {
			t.Fatal("strict Image.RawManifest() opened symlink blob")
		}

		trustedImage, err := l.Reader(WithAllowFinalSymlinks()).Image(manifestDigest)
		if err != nil {
			t.Fatalf("trusted Reader.Image() = %v", err)
		}
		got, err := trustedImage.RawManifest()
		if err != nil {
			t.Fatalf("trusted Image.RawManifest() = %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("trusted Image.RawManifest() differs from symlink target")
		}
	})

	t.Run("config", func(t *testing.T) {
		l := cloneLayoutFixture(t)
		want := replaceLayoutBlobWithSymlink(t, l, configDigest)

		strictImage, err := l.Image(manifestDigest)
		if err != nil {
			t.Fatalf("Path.Image() = %v", err)
		}
		if _, err := strictImage.RawConfigFile(); err == nil {
			t.Fatal("strict Image.RawConfigFile() opened symlink blob")
		}

		trustedImage, err := l.Reader(WithAllowFinalSymlinks()).Image(manifestDigest)
		if err != nil {
			t.Fatalf("trusted Reader.Image() = %v", err)
		}
		got, err := trustedImage.RawConfigFile()
		if err != nil {
			t.Fatalf("trusted Image.RawConfigFile() = %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("trusted Image.RawConfigFile() differs from symlink target")
		}
	})

	t.Run("layer", func(t *testing.T) {
		l := cloneLayoutFixture(t)
		want := replaceLayoutBlobWithSymlink(t, l, readerTestLayerDigest)

		strictImage, err := l.Image(manifestDigest)
		if err != nil {
			t.Fatalf("Path.Image() = %v", err)
		}
		strictLayers, err := strictImage.Layers()
		if err != nil {
			t.Fatalf("strict Image.Layers() = %v", err)
		}
		if rc, err := strictLayers[0].Compressed(); err == nil {
			rc.Close()
			t.Fatal("strict Layer.Compressed() opened symlink blob")
		}

		trustedImage, err := l.Reader(WithAllowFinalSymlinks()).Image(manifestDigest)
		if err != nil {
			t.Fatalf("trusted Reader.Image() = %v", err)
		}
		trustedLayers, err := trustedImage.Layers()
		if err != nil {
			t.Fatalf("trusted Image.Layers() = %v", err)
		}
		rc, err := trustedLayers[0].Compressed()
		if err != nil {
			t.Fatalf("trusted Layer.Compressed() = %v", err)
		}
		got, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			t.Fatalf("ReadAll(trusted layer) = %v", readErr)
		}
		if closeErr != nil {
			t.Fatalf("trusted layer Close() = %v", closeErr)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("trusted Layer.Compressed() differs from symlink target")
		}
	})
}

func TestReaderPoliciesDoNotLeakBetweenConcurrentReaders(t *testing.T) {
	want := []byte("concurrent trusted contents")
	l, h := newFinalSymlinkBlob(t, want)
	strict := l.Reader()
	trusted := l.Reader(WithAllowFinalSymlinks())

	const iterations = 32
	errCh := make(chan error, iterations*2)
	var wg sync.WaitGroup
	for i := 0; i < iterations; i++ {
		wg.Add(2)
		go func(iteration int) {
			defer wg.Done()
			if _, err := strict.Bytes(h); err == nil {
				errCh <- fmt.Errorf("strict reader iteration %d opened symlink blob", iteration)
			}
		}(i)
		go func(iteration int) {
			defer wg.Done()
			got, err := trusted.Bytes(h)
			if err != nil {
				errCh <- fmt.Errorf("trusted reader iteration %d: %w", iteration, err)
				return
			}
			if !bytes.Equal(got, want) {
				errCh <- fmt.Errorf("trusted reader iteration %d read %q, want %q", iteration, got, want)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func newFinalSymlinkBlob(t *testing.T, contents []byte) (Path, v1.Hash) {
	t.Helper()
	l := Path(t.TempDir())
	h := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("c", 64)}
	if err := os.MkdirAll(filepath.Dir(l.blobPath(h)), 0o755); err != nil {
		t.Fatalf("MkdirAll(blobs) = %v", err)
	}
	target := filepath.Join(t.TempDir(), "blob")
	if err := os.WriteFile(target, contents, 0o644); err != nil {
		t.Fatalf("WriteFile(target) = %v", err)
	}
	if err := os.Symlink(target, l.blobPath(h)); err != nil {
		t.Skipf("Symlink() = %v", err)
	}
	return l, h
}

func cloneLayoutFixture(t *testing.T) Path {
	t.Helper()
	destination := t.TempDir()
	if err := filepath.Walk(testPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(testPath, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("fixture entry %s is not a regular file", path)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, contents, info.Mode().Perm())
	}); err != nil {
		t.Fatalf("copy layout fixture = %v", err)
	}
	return Path(destination)
}

func replaceLayoutBlobWithSymlink(t *testing.T, l Path, h v1.Hash) []byte {
	t.Helper()
	blobPath := l.blobPath(h)
	contents, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) = %v", h, err)
	}
	target := filepath.Join(t.TempDir(), h.Hex)
	if err := os.WriteFile(target, contents, 0o644); err != nil {
		t.Fatalf("WriteFile(target %s) = %v", h, err)
	}
	if err := os.Remove(blobPath); err != nil {
		t.Fatalf("Remove(%s) = %v", h, err)
	}
	if err := os.Symlink(target, blobPath); err != nil {
		t.Skipf("Symlink(%s) = %v", h, err)
	}
	return contents
}
