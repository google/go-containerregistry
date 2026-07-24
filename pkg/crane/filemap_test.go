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
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
)

func TestLayer(t *testing.T) {
	tcs := []struct {
		Name    string
		FileMap map[string][]byte
		Digest  string
	}{{
		Name:   "Empty contents",
		Digest: "sha256:89732bc7504122601f40269fc9ddfb70982e633ea9caf641ae45736f2846b004",
	}, {
		Name: "One file",
		FileMap: map[string][]byte{
			"/test": []byte("testy"),
		},
		Digest: "sha256:ec3ff19f471b99a76fb1c339c1dfdaa944a4fba25be6bcdc99fe7e772103079e",
	}, {
		Name: "Two files",
		FileMap: map[string][]byte{
			"/test":    []byte("testy"),
			"/testalt": []byte("footesty"),
		},
		Digest: "sha256:a48bcb7be3ab3ec608ee56eb80901224e19e31dc096cc06a8fd3a8dae1aa8947",
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
		Digest: "sha256:1e637602abbcab2dcedcc24e0b7c19763454a47261f1658b57569530b369ccb9",
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
			if d.String() != tc.Digest {
				t.Errorf("Incorrect digest, want %q, got %q", tc.Digest, d.String())
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
		Digest  string
	}{{
		Name:   "Empty contents",
		Digest: "sha256:5b19422c75ffd2fe40e5cf332959491f16efd4948e5a647dfa18ef0893f60879",
	}, {
		Name: "One file",
		FileMap: map[string][]byte{
			"/test": []byte("testy"),
		},
		Digest: "sha256:0a5a9b92cb817d5362b14d92173e9c2fb813a17f261a6ad55e6a952edf86f21a",
	}, {
		Name: "Two files",
		FileMap: map[string][]byte{
			"/test": []byte("testy"),
			"/bar":  []byte("not useful"),
		},
		Digest: "sha256:d2c82c35a82be1186df962cd85b0eb1ac71f34ff2b9dab125b976cccbf11016c",
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
		Digest: "sha256:9fd84648d36fe2792dfa1b1fadb6ddb31ae0ade5154536013fd912e2ab424616",
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
			if d.String() != tc.Digest {
				t.Fatalf("Incorrect digest, want %q, got %q", tc.Digest, d.String())
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
