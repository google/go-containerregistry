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

package content_test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/content"
)

func TestImageFromContents(t *testing.T) {
	tcs := []struct {
		Name     string
		Contents []map[string][]byte
		Digest   string
	}{{
		Name:   "Empty contents",
		Digest: "sha256:16d76b9d63dda52fc64358c62c715c0a710052de191002a52810229e4c68faec",
	}, {
		Name: "One Layer, one file",
		Contents: []map[string][]byte{{
			"/test": []byte("testy"),
		}},
		Digest: "sha256:d1fd83b38f973d31da3ca7298f9e490e7715c9387bc609cd349ffc3909c20c8a",
	}, {
		Name: "Two Layers, one file",
		Contents: []map[string][]byte{{
			"/test": []byte("testy"),
		}, {
			"/foo": []byte("superuseful"),
		}},
		Digest: "sha256:22785c1917467910bba7396f215556f2a18ccbda0865fe1c3af960517661d0a5",
	}, {
		Name: "One Layer, two files",
		Contents: []map[string][]byte{{
			"/test": []byte("testy"),
			"/bar":  []byte("not useful"),
		}},
		Digest: "sha256:d66dff1eaab5184591bb43a0f7c0ce24ffcab731a38a760e6631431966aaea2b",
	}, {
		Name: "One Layer, many files",
		Contents: []map[string][]byte{{
			"/1": []byte("1"),
			"/2": []byte("2"),
			"/3": []byte("3"),
			"/4": []byte("4"),
			"/5": []byte("5"),
			"/6": []byte("6"),
			"/7": []byte("7"),
			"/8": []byte("8"),
			"/9": []byte("9"),
		}},
		Digest: "sha256:6a79a016f70ff3d574612f7d5ccc4329ee1d573c239e3aeef1e4014fb7294b01",
	}}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			i, err := content.Image(tc.Contents...)
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
			i1, _ := content.Image(tc.Contents...)
			i2, _ := content.Image(tc.Contents...)

			d1, _ := i1.Digest()
			d2, _ := i2.Digest()

			if d1 != d2 {
				t.Fatalf("Non matching digests, want %q, got %q", d1, d2)
			}

		})
	}
}
