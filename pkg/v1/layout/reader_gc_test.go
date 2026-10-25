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
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

func TestReaderGarbageCollectFinalSymlinkPolicy(t *testing.T) {
	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("random.Image() = %v", err)
	}
	l, err := Write(t.TempDir(), empty.Index)
	if err != nil {
		t.Fatalf("Write() = %v", err)
	}
	if err := l.AppendImage(img); err != nil {
		t.Fatalf("AppendImage() = %v", err)
	}
	digest, err := img.Digest()
	if err != nil {
		t.Fatalf("image Digest() = %v", err)
	}
	replaceLayoutBlobWithSymlink(t, l, digest)

	if _, err := l.GarbageCollect(); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("Path.GarbageCollect() error = %v, want final-symlink rejection", err)
	}
	if _, err := l.Reader(WithAllowFinalSymlinks()).GarbageCollect(); err != nil {
		t.Fatalf("trusted Reader.GarbageCollect() = %v", err)
	}
	if _, err := l.GarbageCollect(); err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("Path.GarbageCollect() after trusted call error = %v, want unchanged strict behavior", err)
	}
}
