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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func TestOpenBlobRejectsSymlink(t *testing.T) {
	l := Path(t.TempDir())
	regularHash := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("a", 64)}
	regularContents := []byte("regular blob")

	if err := os.MkdirAll(filepath.Dir(l.blobPath(regularHash)), 0o755); err != nil {
		t.Fatalf("MkdirAll() = %v", err)
	}
	if err := os.WriteFile(l.blobPath(regularHash), regularContents, 0o644); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	f, err := l.openBlob(regularHash)
	if err != nil {
		t.Fatalf("openBlob() = %v", err)
	}
	got, err := io.ReadAll(f)
	if err != nil {
		f.Close()
		t.Fatalf("ReadAll() = %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if !bytes.Equal(got, regularContents) {
		t.Fatalf("openBlob() read %q, want %q", got, regularContents)
	}

	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("symlink target"), 0o644); err != nil {
		t.Fatalf("WriteFile(outside) = %v", err)
	}
	symlinkHash := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("b", 64)}
	if err := os.Symlink(outside, l.blobPath(symlinkHash)); err != nil {
		t.Skipf("Symlink() = %v", err)
	}

	if f, err := l.openBlob(symlinkHash); err == nil {
		f.Close()
		t.Fatalf("openBlob() opened symlink blob")
	}
}
