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
	}{
		{
			Name:   "Empty contents",
			Digest: "sha256:16d76b9d63dda52fc64358c62c715c0a710052de191002a52810229e4c68faec",
		},
		{
			Name: "One Layer, one file",
			Contents: []map[string][]byte{
				{
					"/test": []byte("testy"),
				},
			},
			Digest: "sha256:d1fd83b38f973d31da3ca7298f9e490e7715c9387bc609cd349ffc3909c20c8a",
		},
		{
			Name: "Two Layers, one file",
			Contents: []map[string][]byte{
				{
					"/test": []byte("testy"),
				},
				{
					"/foo": []byte("superuseful"),
				},
			},
			Digest: "sha256:22785c1917467910bba7396f215556f2a18ccbda0865fe1c3af960517661d0a5",
		},
		{
			Name: "One Layer, two files",
			Contents: []map[string][]byte{
				{
					"/test": []byte("testy"),
					"/bar":  []byte("not useful"),
				},
			},
			Digest: "sha256:87b8a07dd2304fdb2180f9b2daebe2731cc66da3984d34623666da4ae0998419",
		},
	}
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
	}
}
