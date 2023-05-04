// Copyright 2023 Google LLC All Rights Reserved.
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

package test_test

import (
	"testing"

	"github.com/google/go-containerregistry/internal/test"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestTest(t *testing.T) {
	must := test.T(t)
	s := "gcr.io/foo/bar"

	t.Log(must.Digest(must.Image(random.Image(1024, 5))))
	t.Log(must.Digest(must.Index(random.Index(1024, 5, 5))))
	t.Log(must.ParseReference(s).Context())
	t.Log(must.Tag(s).Context())

	// A generic but more verbose way to do the same thing:

	t.Log(test.Must[v1.Hash](t).Do(
		test.Must[v1.Image](t).Do(random.Image(1024, 5)).Digest(),
	))
	t.Log(test.Must[v1.Hash](t).Do(
		test.Must[v1.ImageIndex](t).Do(random.Index(1024, 5, 5)).Digest(),
	))
	t.Log(test.Must[name.Reference](t).Do(name.ParseReference(s)).Context())
	t.Log(test.Must[name.Reference](t).Do(name.NewTag(s)).Context())

	// But the generic way works for anything.
	t.Log(test.Must[*remote.Descriptor](t).Do(remote.Get(
		must.ParseReference("alpine"),
	)).Digest)
}
