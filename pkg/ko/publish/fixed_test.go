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

package publish

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
)

var (
	fixedBaseRepo, _ = name.NewRepository("gcr.io/asdf", name.WeakValidation)
)

func TestFixed(t *testing.T) {
	hex1 := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	hex2 := "baadf00dbaadf00dbaadf00dbaadf00dbaadf00dbaadf00dbaadf00dbaadf00d"
	f := NewFixed(fixedBaseRepo, map[string]v1.Hash{
		"foo": v1.Hash{
			Algorithm: "sha256",
			Hex:       hex1,
		},
		"bar": v1.Hash{
			Algorithm: "sha256",
			Hex:       hex2,
		},
	})

	fooDigest, err := f.Publish(nil, "foo")
	if err != nil {
		t.Errorf("Publish(foo) = %v", err)
	}
	if got, want := fooDigest.String(), "gcr.io/asdf/foo@sha256:"+hex1; got != want {
		t.Errorf("Publish(foo) = %q, want %q", got, want)
	}

	barDigest, err := f.Publish(nil, "bar")
	if err != nil {
		t.Errorf("Publish(bar) = %v", err)
	}
	if got, want := barDigest.String(), "gcr.io/asdf/bar@sha256:"+hex2; got != want {
		t.Errorf("Publish(bar) = %q, want %q", got, want)
	}

	d, err := f.Publish(nil, "baz")
	if err == nil {
		t.Errorf("Publish(baz) = %v, want error", d)
	}
}
