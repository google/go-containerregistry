// Copyright 2021 Google LLC All Rights Reserved.
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

package static

import (
	"io"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestNewLayer(t *testing.T) {
	b := []byte(strings.Repeat(".", 10))
	l := NewLayer(b, types.OCILayer)

	// This does basically nothing.
	if err := validate.Layer(l, validate.Fast); err != nil {
		t.Fatal(err)
	}

	// Digest and DiffID match, and match expectations.
	h, err := l.Digest()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := l.DiffID()
	if err != nil {
		t.Fatal(err)
	}
	if h != h2 {
		t.Errorf("Digest != DiffID; digest is %v, diffid is %v", h, h2)
	}
	wantDigest, err := v1.NewHash("sha256:537f3fb69ba01fc388a3a5c920c485b2873d5f327305c3dd2004d6a04451659b")
	if err != nil {
		t.Fatal(err)
	}
	if h != wantDigest {
		t.Errorf("Digest mismatch; got %v, want %v", h, wantDigest)
	}

	sz, err := l.Size()
	if err != nil {
		t.Fatal(err)
	}
	if sz != 10 {
		t.Errorf("Size mismatch; got %d, want %d", sz, 10)
	}

	mt, err := l.MediaType()
	if err != nil {
		t.Fatal(err)
	}
	if mt != types.OCILayer {
		t.Errorf("MediaType mismatch; got %v, want %v", mt, types.OCILayer)
	}

	r, err := l.Uncompressed()
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(b) {
		t.Errorf("Contents mismatch: got %q, want %q", string(got), string(b))
	}
}
