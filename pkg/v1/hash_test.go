// Copyright 2018 Google LLC All Rights Reserved.
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

package v1

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestGoodHashes(t *testing.T) {
	good := []string{
		"sha256:deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	for _, s := range good {
		h, err := NewHash(s)
		if err != nil {
			t.Error("Unexpected error parsing hash:", err)
		}
		if got, want := h.String(), s; got != want {
			t.Errorf("String(); got %q, want %q", got, want)
		}
		bytes, err := json.Marshal(h)
		if err != nil {
			t.Error("Unexpected error json.Marshaling hash:", err)
		}
		if got, want := string(bytes), strconv.Quote(h.String()); got != want {
			t.Errorf("json.Marshal(); got %q, want %q", got, want)
		}
	}
}

func TestBadHashes(t *testing.T) {
	bad := []string{
		// Too short
		"sha256:deadbeef",
		// Bad character
		"sha256:o123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		// Unknown algorithm
		"md5:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		// Too few parts
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		// Too many parts
		"md5:sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	for _, s := range bad {
		h, err := NewHash(s)
		if err == nil {
			t.Error("Expected error, got:", h)
		}
	}
}

func TestSHA256(t *testing.T) {
	input := "asdf"
	h, n, err := SHA256(strings.NewReader(input))
	if err != nil {
		t.Error("SHA256(asdf) =", err)
	}
	if got, want := h.Algorithm, "sha256"; got != want {
		t.Errorf("Algorithm; got %v, want %v", got, want)
	}
	if got, want := h.Hex, "f0e4c2f76c58916ec258f246851bea091d14d4247a2fc3e18694461b1816e13b"; got != want {
		t.Errorf("Hex; got %v, want %v", got, want)
	}
	if got, want := n, int64(len(input)); got != want {
		t.Errorf("n; got %v, want %v", got, want)
	}
}

// This tests that you can use Hash as a key in a map (needs to implement both
// MarshalText and UnmarshalText).
func TestTextMarshalling(t *testing.T) {
	foo := make(map[Hash]string)
	b, err := json.Marshal(foo)
	if err != nil {
		t.Fatal("could not marshal:", err)
	}
	if err := json.Unmarshal(b, &foo); err != nil {
		t.Error("could not unmarshal:", err)
	}

	h := &Hash{
		Algorithm: "sha256",
		Hex:       strings.Repeat("a", 64),
	}
	g := &Hash{}
	text, err := h.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if err := g.UnmarshalText(text); err != nil {
		t.Fatal(err)
	}

	if h.String() != g.String() {
		t.Errorf("mismatched hash: %s != %s", h, g)
	}
}
