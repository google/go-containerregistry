// Copyright 2022 Google LLC All Rights Reserved.
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

package remote_test

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestReferrers_FallbackTag(t *testing.T) {
	// Set up a fake registry that doesn't support the Referrers API.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	descriptor := func(img v1.Image) v1.Descriptor {
		d, err := img.Digest()
		if err != nil {
			t.Fatal(err)
		}
		sz, err := img.Size()
		if err != nil {
			t.Fatal(err)
		}
		mt, err := img.MediaType()
		if err != nil {
			t.Fatal(err)
		}
		return v1.Descriptor{
			Digest:    d,
			Size:      sz,
			MediaType: mt,
		}
	}

	// Push an image we'll attach things to.
	// We'll copy from src to dst.
	rootRef, err := name.ParseReference(fmt.Sprintf("%s/repo:root", u.Host))
	if err != nil {
		t.Fatal(err)
	}
	rootImg, err := random.Image(10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(rootRef, rootImg); err != nil {
		t.Fatal(err)
	}
	rootDesc := descriptor(rootImg)
	t.Logf("root image is %s", rootDesc.Digest)

	// Push an image that refers to the root image as its subject.
	leafRef, err := name.ParseReference(fmt.Sprintf("%s/repo:leaf", u.Host))
	if err != nil {
		t.Fatal(err)
	}
	leafImg, err := random.Image(20, 20)
	if err != nil {
		t.Fatal(err)
	}
	leafImg = mutate.Subject(leafImg, rootDesc).(v1.Image)
	if err := remote.Write(leafRef, leafImg); err != nil {
		t.Fatal(err)
	}
	leafDesc := descriptor(leafImg)
	t.Logf("leaf image is %s", leafDesc.Digest)

	// Get the referrers of the root image, by digest.
	rootRefDigest := rootRef.Context().Digest(rootDesc.Digest.String())
	referrers, err := remote.Referrers(rootRefDigest)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff([]v1.Descriptor{leafDesc}, referrers); d != "" {
		t.Fatalf("referrers diff (-want,+got): %s", d)
	}

	// Get the referrers by querying the root image's fallback tag directly.
	tag, err := name.ParseReference(fmt.Sprintf("%s/repo:sha256-%s", u.Host, rootDesc.Digest.Hex))
	if err != nil {
		t.Fatal(err)
	}
	idx, err := remote.Index(tag)
	if err != nil {
		t.Fatal(err)
	}
	mf, err := idx.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(referrers, mf.Manifests); d != "" {
		t.Fatalf("fallback tag diff (-want,+got): %s", d)
	}

	// Push the leaf image again, this time with a different tag.
	// This shouldn't add another item to the root image's referrers,
	// because it's the same digest.
	// Push an image that refers to the root image as its subject.
	leaf2Ref, err := name.ParseReference(fmt.Sprintf("%s/repo:leaf2", u.Host))
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(leaf2Ref, leafImg); err != nil {
		t.Fatal(err)
	}
	// Get the referrers of the root image again, which should only have one entry.
	rootRefDigest = rootRef.Context().Digest(rootDesc.Digest.String())
	referrers, err = remote.Referrers(rootRefDigest)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff([]v1.Descriptor{leafDesc}, referrers); d != "" {
		t.Fatalf("referrers diff after second push (-want,+got): %s", d)
	}
}
