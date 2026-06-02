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

package limit

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestReadAll(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		max     int64
		want    string
		wantErr bool
	}{
		{name: "under limit", input: "hello", max: 10, want: "hello"},
		{name: "at limit", input: "hello", max: 5, want: "hello"},
		{name: "over limit", input: "hello world", max: 5, wantErr: true},
		{name: "empty under limit", input: "", max: 5, want: ""},
		{name: "empty zero limit", input: "", max: 0, want: ""},
		{name: "one byte over zero limit", input: "x", max: 0, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadAll(bytes.NewReader([]byte(tc.input)), tc.max)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestReadAllPropagatesError(t *testing.T) {
	want := errors.New("boom")
	if _, err := ReadAll(errReader{err: want}, 10); !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
}

func TestReadAllReadsExactlyMax(t *testing.T) {
	// Ensure we don't return after reading max+1 bytes from a long stream.
	r := io.MultiReader(bytes.NewReader(bytes.Repeat([]byte("a"), 1024)), errReader{err: errors.New("should not be read")})
	if _, err := ReadAll(r, 10); err == nil {
		t.Fatal("expected over-limit error")
	}
}
