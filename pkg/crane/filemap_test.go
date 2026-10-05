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

package crane_test

import (
	"archive/tar"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
)

func TestLayer(t *testing.T) {
	tcs := []struct {
		Name    string
		FileMap map[string][]byte
		Digests []string
	}{{
		Name: "Empty contents",
		Digests: []string{
			"sha256:89732bc7504122601f40269fc9ddfb70982e633ea9caf641ae45736f2846b004",
			"sha256:f1a96f347ed7b559984a3f22f92e02007aec407909b05639910e54cb91fa9b1e", // Go 1.27+
		},
	}, {
		Name: "One file",
		FileMap: map[string][]byte{
			"/test": []byte("testy"),
		},
		Digests: []string{
			"sha256:ec3ff19f471b99a76fb1c339c1dfdaa944a4fba25be6bcdc99fe7e772103079e",
			"sha256:73a212a976527f322e80b3e429215dda1d339681e5335ae3a4019ff51124302d", // Go 1.27+
		},
	}, {
		Name: "Two files",
		FileMap: map[string][]byte{
			"/test":    []byte("testy"),
			"/testalt": []byte("footesty"),
		},
		Digests: []string{
			"sha256:a48bcb7be3ab3ec608ee56eb80901224e19e31dc096cc06a8fd3a8dae1aa8947",
			"sha256:3a0b79b547c0093da15681ad44a34316bcc2e66d90c6779d0a574fe99334f2ef", // Go 1.27+
		},
	}, {
		Name: "Many files",
		FileMap: map[string][]byte{
			"/1": []byte("1"),
			"/2": []byte("2"),
			"/3": []byte("3"),
			"/4": []byte("4"),
			"/5": []byte("5"),
			"/6": []byte("6"),
			"/7": []byte("7"),
			"/8": []byte("8"),
			"/9": []byte("9"),
		},
		Digests: []string{
			"sha256:1e637602abbcab2dcedcc24e0b7c19763454a47261f1658b57569530b369ccb9",
			"sha256:297b435d5e33ffb30cf6a5e52927148e2f9ca7b66207c66e42bfd94851d9a2f8", // Go 1.27+
		},
	}}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			l, err := crane.Layer(tc.FileMap)
			if err != nil {
				t.Fatalf("Error calling layer: %v", err)
			}

			d, err := l.Digest()
			if err != nil {
				t.Fatalf("Error calling digest: %v", err)
			}
			if !slices.Contains(tc.Digests, d.String()) {
				t.Errorf("Incorrect digest, got %q, want one of %v", d.String(), tc.Digests)
			}

			// Check contents match.
			rc, err := l.Uncompressed()
			if err != nil {
				t.Fatalf("Uncompressed: %v", err)
			}
			defer rc.Close()
			tr := tar.NewReader(rc)
			saw := map[string]struct{}{}
			for {
				th, err := tr.Next()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatalf("Next: %v", err)
				}
				saw[th.Name] = struct{}{}
				want, found := tc.FileMap[th.Name]
				if !found {
					t.Errorf("found %q, not in original map", th.Name)
					continue
				}
				got, err := io.ReadAll(tr)
				if err != nil {
					t.Fatalf("ReadAll(%q): %v", th.Name, err)
				}
				if string(want) != string(got) {
					t.Errorf("File %q: got %v, want %v", th.Name, string(got), string(want))
				}
			}
			for k := range saw {
				delete(tc.FileMap, k)
			}
			for k := range tc.FileMap {
				t.Errorf("Layer did not contain %q", k)
			}
		})
		t.Run(tc.Name+" is reproducible", func(t *testing.T) {
			l1, _ := crane.Layer(tc.FileMap)
			l2, _ := crane.Layer(tc.FileMap)
			d1, _ := l1.Digest()
			d2, _ := l2.Digest()
			if d1 != d2 {
				t.Fatalf("Non matching digests, want %q, got %q", d1, d2)
			}
		})
	}
}

func TestImage(t *testing.T) {
	tcs := []struct {
		Name    string
		FileMap map[string][]byte
		Digests []string
	}{{
		Name: "Empty contents",
		Digests: []string{
			"sha256:98132f58b523c391a5788997327cac95e114e3a6609d01163189774510705399",
			"sha256:a4f44966d997cdf300821826390d0f85396ab2fc71542ef9839e34b322d766e4", // Go 1.27+
		},
	}, {
		Name: "One file",
		FileMap: map[string][]byte{
			"/test": []byte("testy"),
		},
		Digests: []string{
			"sha256:d905c03ac635172a96c12b8af6c90cfd028e3edaa3114b31a9e196ab38c16963",
			"sha256:c97f08502dd67add5f98a510ee366aedce248b194b7c9f2c189b3336edb6bd7b", // Go 1.27+
		},
	}, {
		Name: "Two files",
		FileMap: map[string][]byte{
			"/test": []byte("testy"),
			"/bar":  []byte("not useful"),
		},
		Digests: []string{
			"sha256:20e7e4800e5eb167f170970936c08d9e1bcbe91372420eeb6ab8d1a07752c3a3",
			"sha256:a7f4dc797069f40d3180775e1e813dbbf8372c68b264522ebebe1018a2ad8230", // Go 1.27+
		},
	}, {
		Name: "Many files",
		FileMap: map[string][]byte{
			"/1": []byte("1"),
			"/2": []byte("2"),
			"/3": []byte("3"),
			"/4": []byte("4"),
			"/5": []byte("5"),
			"/6": []byte("6"),
			"/7": []byte("7"),
			"/8": []byte("8"),
			"/9": []byte("9"),
		},
		Digests: []string{
			"sha256:dfca2803510c8e3b83a3151f7c035c60cfa2a8a52465b802e18b85014de361f1",
			"sha256:b99859055a07ff6e6a4fbbbe76e35e4060804f1de15d036ee4ed906539f64ce6", // Go 1.27+
		},
	}}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			i, err := crane.Image(tc.FileMap)
			if err != nil {
				t.Fatalf("Error calling image: %v", err)
			}
			d, err := i.Digest()
			if err != nil {
				t.Fatalf("Error calling digest: %v", err)
			}
			if !slices.Contains(tc.Digests, d.String()) {
				t.Fatalf("Incorrect digest, got %q, want one of %v", d.String(), tc.Digests)
			}
		})
		t.Run(tc.Name+" is reproducible", func(t *testing.T) {
			i1, _ := crane.Image(tc.FileMap)
			i2, _ := crane.Image(tc.FileMap)
			d1, _ := i1.Digest()
			d2, _ := i2.Digest()
			if d1 != d2 {
				t.Fatalf("Non matching digests, want %q, got %q", d1, d2)
			}
		})
	}
}
