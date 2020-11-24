// Copyright 2020 Google LLC All Rights Reserved.
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

package match_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/types"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestName(t *testing.T) {
	tests := []struct {
		desc  v1.Descriptor
		name  string
		match bool
	}{
		{v1.Descriptor{Annotations: map[string]string{imagespec.AnnotationRefName: "foo"}}, "foo", true},
		{v1.Descriptor{Annotations: map[string]string{imagespec.AnnotationRefName: "foo"}}, "bar", false},
		{v1.Descriptor{Annotations: map[string]string{}}, "bar", false},
		{v1.Descriptor{Annotations: nil}, "bar", false},
		{v1.Descriptor{}, "bar", false},
	}
	for i, tt := range tests {
		f := match.Name(tt.name)
		if match := f(tt.desc); match != tt.match {
			t.Errorf("%d: mismatched, got %v expected %v for desc %#v name %s", i, match, tt.match, tt.desc, tt.name)
		}
	}
}

func TestAnnotation(t *testing.T) {
	tests := []struct {
		desc  v1.Descriptor
		key   string
		value string
		match bool
	}{
		{v1.Descriptor{Annotations: map[string]string{"foo": "bar"}}, "foo", "bar", true},
		{v1.Descriptor{Annotations: map[string]string{"foo": "bar"}}, "bar", "foo", false},
		{v1.Descriptor{Annotations: map[string]string{}}, "foo", "bar", false},
		{v1.Descriptor{Annotations: nil}, "foo", "bar", false},
		{v1.Descriptor{}, "foo", "bar", false},
	}
	for i, tt := range tests {
		f := match.Annotation(tt.key, tt.value)
		if match := f(tt.desc); match != tt.match {
			t.Errorf("%d: mismatched, got %v expected %v for desc %#v annotation %s:%s", i, match, tt.match, tt.desc, tt.key, tt.value)
		}
	}
}

func TestPlatforms(t *testing.T) {
	tests := []struct {
		desc      v1.Descriptor
		platforms []v1.Platform
		match     bool
	}{
		{v1.Descriptor{Platform: &v1.Platform{Architecture: "amd64", OS: "linux"}}, []v1.Platform{{Architecture: "amd64", OS: "darwin"}, {Architecture: "amd64", OS: "linux"}}, true},
		{v1.Descriptor{Platform: &v1.Platform{Architecture: "amd64", OS: "linux"}}, []v1.Platform{{Architecture: "arm64", OS: "linux"}, {Architecture: "s390x", OS: "linux"}}, false},
		{v1.Descriptor{Platform: &v1.Platform{OS: "linux"}}, []v1.Platform{{Architecture: "arm64", OS: "linux"}}, false},
		{v1.Descriptor{Platform: &v1.Platform{}}, []v1.Platform{{Architecture: "arm64", OS: "linux"}}, false},
		{v1.Descriptor{Platform: nil}, []v1.Platform{{Architecture: "arm64", OS: "linux"}}, false},
		{v1.Descriptor{}, []v1.Platform{{Architecture: "arm64", OS: "linux"}}, false},
	}
	for i, tt := range tests {
		f := match.Platforms(tt.platforms...)
		if match := f(tt.desc); match != tt.match {
			t.Errorf("%d: mismatched, got %v expected %v for desc %#v platform %#v", i, match, tt.match, tt.desc, tt.platforms)
		}
	}
}

func TestMediaTypes(t *testing.T) {
	tests := []struct {
		desc       v1.Descriptor
		mediaTypes []string
		match      bool
	}{
		{v1.Descriptor{MediaType: types.OCIImageIndex}, []string{string(types.OCIImageIndex)}, true},
		{v1.Descriptor{MediaType: types.OCIImageIndex}, []string{string(types.OCIManifestSchema1)}, false},
		{v1.Descriptor{MediaType: types.OCIImageIndex}, []string{string(types.OCIManifestSchema1), string(types.OCIImageIndex)}, true},
		{v1.Descriptor{MediaType: types.OCIImageIndex}, []string{"a", "b"}, false},
		{v1.Descriptor{}, []string{string(types.OCIManifestSchema1), string(types.OCIImageIndex)}, false},
	}
	for i, tt := range tests {
		f := match.MediaTypes(tt.mediaTypes...)
		if match := f(tt.desc); match != tt.match {
			t.Errorf("%d: mismatched, got %v expected %v for desc %#v mediaTypes %#v", i, match, tt.match, tt.desc, tt.mediaTypes)
		}
	}
}

func TestDigests(t *testing.T) {
	hashes := []string{
		"deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"abcde1111111222f0123456789abcdef0123456789abcdef0123456789abcdef",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	algo := "sha256"

	tests := []struct {
		desc    v1.Descriptor
		digests []v1.Hash
		match   bool
	}{
		{v1.Descriptor{Digest: v1.Hash{Algorithm: algo, Hex: hashes[0]}}, []v1.Hash{{Algorithm: algo, Hex: hashes[0]}, {Algorithm: algo, Hex: hashes[1]}}, true},
		{v1.Descriptor{Digest: v1.Hash{Algorithm: algo, Hex: hashes[1]}}, []v1.Hash{{Algorithm: algo, Hex: hashes[0]}, {Algorithm: algo, Hex: hashes[1]}}, true},
		{v1.Descriptor{Digest: v1.Hash{Algorithm: algo, Hex: hashes[2]}}, []v1.Hash{{Algorithm: algo, Hex: hashes[0]}, {Algorithm: algo, Hex: hashes[1]}}, false},
	}
	for i, tt := range tests {
		f := match.Digests(tt.digests...)
		if match := f(tt.desc); match != tt.match {
			t.Errorf("%d: mismatched, got %v expected %v for desc %#v digests %#v", i, match, tt.match, tt.desc, tt.digests)
		}
	}
}
