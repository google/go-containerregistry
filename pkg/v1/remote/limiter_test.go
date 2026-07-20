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

package remote

import (
	"io"
	"strings"
	"testing"
)

func TestLimitedReadCloserReleasesOnEOF(t *testing.T) {
	released := false
	lrc := &limitedReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader("some data")),
		release:    func() { released = true },
	}

	if _, err := io.Copy(io.Discard, lrc); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	if !released {
		t.Error("release was not called after reading to EOF without Close")
	}
}

func TestLimitedReadCloserReleasesOnce(t *testing.T) {
	count := 0
	lrc := &limitedReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader("some data")),
		release:    func() { count++ },
	}

	if _, err := io.Copy(io.Discard, lrc); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	if err := lrc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if count != 1 {
		t.Errorf("release called %d times, want 1", count)
	}
}
