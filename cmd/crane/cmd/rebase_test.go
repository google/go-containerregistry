// Copyright 2026 Google LLC All Rights Reserved.
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

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// TestRebaseIndex rebases a multi-platform index. It first rebases with
// explicit --old_base and --new_base, then with the base image annotations that
// crane mutate sets on the index.
func TestRebaseIndex(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	amd64 := v1.Platform{OS: "linux", Architecture: "amd64"}
	arm64 := v1.Platform{OS: "linux", Architecture: "arm64"}
	child := func(img v1.Image, p v1.Platform) mutate.IndexAddendum {
		return mutate.IndexAddendum{Add: img, Descriptor: v1.Descriptor{Platform: &p}}
	}

	// old_base and new_base each have one image per platform. orig is built on
	// old_base: each platform image gets one extra top layer.
	oldAmd64, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	oldArm64, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	oldIdx := mutate.AppendManifests(empty.Index, child(oldAmd64, amd64), child(oldArm64, arm64))

	topAmd64, err := random.Layer(1024, types.DockerLayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}
	topArm64, err := random.Layer(1024, types.DockerLayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}
	origAmd64, err := mutate.AppendLayers(oldAmd64, topAmd64)
	if err != nil {
		t.Fatalf("mutate.AppendLayers: %v", err)
	}
	origArm64, err := mutate.AppendLayers(oldArm64, topArm64)
	if err != nil {
		t.Fatalf("mutate.AppendLayers: %v", err)
	}
	origIdx := mutate.AppendManifests(empty.Index, child(origAmd64, amd64), child(origArm64, arm64))

	newAmd64, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	newArm64, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	newIdx := mutate.AppendManifests(empty.Index, child(newAmd64, amd64), child(newArm64, arm64))

	oldRef := fmt.Sprintf("%s/test/base:old", u.Host)
	newRef := fmt.Sprintf("%s/test/base:new", u.Host)
	origRef := fmt.Sprintf("%s/test/app:latest", u.Host)
	for _, p := range []struct {
		ref string
		idx v1.ImageIndex
	}{{oldRef, oldIdx}, {newRef, newIdx}, {origRef, origIdx}} {
		r, err := name.ParseReference(p.ref)
		if err != nil {
			t.Fatalf("name.ParseReference: %v", err)
		}
		if err := remote.WriteIndex(r, p.idx); err != nil {
			t.Fatalf("remote.WriteIndex(%s): %v", p.ref, err)
		}
	}

	oldBaseDigest, err := oldIdx.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	newBaseDigest, err := newIdx.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	topAmd64Digest, err := topAmd64.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	topArm64Digest, err := topArm64.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	// Each rebased platform image is new_base's layers for that platform, then
	// orig's top layer. The old base layers must be gone.
	want := map[string][]v1.Hash{
		amd64.String(): append(layerDigests(t, newAmd64), topAmd64Digest),
		arm64.String(): append(layerDigests(t, newArm64), topArm64Digest),
	}

	// check verifies a rebased index at ref: its base image annotations, that
	// each child keeps its platform and has the expected layers, and that each
	// child carries the same base image annotations.
	check := func(ref string, baseName string, baseDigest v1.Hash) {
		t.Helper()
		r, err := name.ParseReference(ref)
		if err != nil {
			t.Fatalf("name.ParseReference: %v", err)
		}
		idx, err := remote.Index(r)
		if err != nil {
			t.Fatalf("remote.Index(%s): %v", ref, err)
		}
		m, err := idx.IndexManifest()
		if err != nil {
			t.Fatalf("IndexManifest: %v", err)
		}
		if got := m.Annotations[specsv1.AnnotationBaseImageName]; got != baseName {
			t.Errorf("index %s = %q, want %q", specsv1.AnnotationBaseImageName, got, baseName)
		}
		if got := m.Annotations[specsv1.AnnotationBaseImageDigest]; got != baseDigest.String() {
			t.Errorf("index %s = %q, want %q", specsv1.AnnotationBaseImageDigest, got, baseDigest)
		}
		if len(m.Manifests) != len(want) {
			t.Fatalf("rebased index has %d manifests, want %d", len(m.Manifests), len(want))
		}
		for _, desc := range m.Manifests {
			if desc.Platform == nil {
				t.Errorf("child %s lost its platform", desc.Digest)
				continue
			}
			key := desc.Platform.String()
			img, err := remote.Image(r, remote.WithPlatform(*desc.Platform))
			if err != nil {
				t.Fatalf("remote.Image(%s, %s): %v", ref, key, err)
			}
			if d := cmp.Diff(want[key], layerDigests(t, img)); d != "" {
				t.Errorf("%s layers (-want,+got): %s", key, d)
			}
			cm, err := img.Manifest()
			if err != nil {
				t.Fatalf("Manifest: %v", err)
			}
			if got := cm.Annotations[specsv1.AnnotationBaseImageName]; got != baseName {
				t.Errorf("%s %s = %q, want %q", key, specsv1.AnnotationBaseImageName, got, baseName)
			}
			if got := cm.Annotations[specsv1.AnnotationBaseImageDigest]; got != baseDigest.String() {
				t.Errorf("%s %s = %q, want %q", key, specsv1.AnnotationBaseImageDigest, got, baseDigest)
			}
		}
	}

	options := []crane.Option{}

	// Explicit --old_base and --new_base.
	rebasedRef := fmt.Sprintf("%s/test/app:rebased", u.Host)
	explicit := NewCmdRebase(&options)
	var out bytes.Buffer
	explicit.SetArgs([]string{origRef, "--old_base", oldRef, "--new_base", newRef, "-t", rebasedRef})
	explicit.SetOut(&out)
	explicit.SetErr(io.Discard)
	if err := explicit.Execute(); err != nil {
		t.Fatalf("rebase with explicit bases: %v", err)
	}
	check(rebasedRef, newRef, newBaseDigest)

	rebasedName, err := name.ParseReference(rebasedRef)
	if err != nil {
		t.Fatalf("name.ParseReference: %v", err)
	}
	rebasedIdx, err := remote.Index(rebasedName)
	if err != nil {
		t.Fatalf("remote.Index: %v", err)
	}
	rebasedDigest, err := rebasedIdx.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if want, got := rebasedName.Context().Digest(rebasedDigest.String()).String(), strings.TrimSpace(out.String()); got != want {
		t.Errorf("printed digest reference = %q, want %q", got, want)
	}

	// Record the bases on orig with crane mutate, then rebase without flags.
	annotate := NewCmdMutate(&options)
	annotate.SetArgs([]string{
		origRef,
		"-a", "org.opencontainers.image.base.name=" + newRef,
		"-a", "org.opencontainers.image.base.digest=" + oldBaseDigest.String(),
	})
	annotate.SetOut(io.Discard)
	annotate.SetErr(io.Discard)
	if err := annotate.Execute(); err != nil {
		t.Fatalf("mutate orig base annotations: %v", err)
	}

	implicitRef := fmt.Sprintf("%s/test/app:implicit", u.Host)
	implicit := NewCmdRebase(&options)
	implicit.SetArgs([]string{origRef, "-t", implicitRef})
	implicit.SetOut(io.Discard)
	implicit.SetErr(io.Discard)
	if err := implicit.Execute(); err != nil {
		t.Fatalf("rebase using base annotations: %v", err)
	}
	check(implicitRef, newRef, newBaseDigest)
}

// layerDigests returns the digests of img's layers, in order.
func layerDigests(t *testing.T, img v1.Image) []v1.Hash {
	t.Helper()
	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}
	digests := make([]v1.Hash, 0, len(layers))
	for _, l := range layers {
		d, err := l.Digest()
		if err != nil {
			t.Fatalf("Digest: %v", err)
		}
		digests = append(digests, d)
	}
	return digests
}

// TestRebaseIndexInlineChildData rebases an index whose child descriptor carries
// inline manifest data. The rebased child's inline data must match its new
// digest, or reading the rebased index back fails verification.
func TestRebaseIndexInlineChildData(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	amd64 := v1.Platform{OS: "linux", Architecture: "amd64"}

	oldImg, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	top, err := random.Layer(1024, types.DockerLayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}
	origImg, err := mutate.AppendLayers(oldImg, top)
	if err != nil {
		t.Fatalf("mutate.AppendLayers: %v", err)
	}
	origRaw, err := origImg.RawManifest()
	if err != nil {
		t.Fatalf("RawManifest: %v", err)
	}
	newImg, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	topDigest, err := top.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}

	oldIdx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: oldImg, Descriptor: v1.Descriptor{Platform: &amd64}})
	newIdx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: newImg, Descriptor: v1.Descriptor{Platform: &amd64}})
	// The child's inline data is its raw manifest, so it matches the child digest.
	origIdx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: origImg, Descriptor: v1.Descriptor{Platform: &amd64, Data: origRaw}})

	oldRef := fmt.Sprintf("%s/test/data-base:old", u.Host)
	newRef := fmt.Sprintf("%s/test/data-base:new", u.Host)
	origRef := fmt.Sprintf("%s/test/data-app:latest", u.Host)
	for _, p := range []struct {
		ref string
		idx v1.ImageIndex
	}{{oldRef, oldIdx}, {newRef, newIdx}, {origRef, origIdx}} {
		r, err := name.ParseReference(p.ref)
		if err != nil {
			t.Fatalf("name.ParseReference: %v", err)
		}
		if err := remote.WriteIndex(r, p.idx); err != nil {
			t.Fatalf("remote.WriteIndex(%s): %v", p.ref, err)
		}
	}

	options := []crane.Option{}
	rebasedRef := fmt.Sprintf("%s/test/data-app:rebased", u.Host)
	rebaseCmd := NewCmdRebase(&options)
	rebaseCmd.SetArgs([]string{origRef, "--old_base", oldRef, "--new_base", newRef, "-t", rebasedRef})
	rebaseCmd.SetOut(io.Discard)
	rebaseCmd.SetErr(io.Discard)
	if err := rebaseCmd.Execute(); err != nil {
		t.Fatalf("rebase: %v", err)
	}

	// Read the rebased index back and resolve its child. Resolving checks the
	// inline data against the child digest.
	rebasedName, err := name.ParseReference(rebasedRef)
	if err != nil {
		t.Fatalf("name.ParseReference: %v", err)
	}
	rebasedIdx, err := remote.Index(rebasedName)
	if err != nil {
		t.Fatalf("remote.Index: %v", err)
	}
	m, err := rebasedIdx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest: %v", err)
	}
	if len(m.Manifests) != 1 {
		t.Fatalf("rebased index has %d manifests, want 1", len(m.Manifests))
	}
	child := m.Manifests[0]
	img, err := rebasedIdx.Image(child.Digest)
	if err != nil {
		t.Fatalf("resolving rebased child %s: %v", child.Digest, err)
	}
	raw, err := img.RawManifest()
	if err != nil {
		t.Fatalf("RawManifest: %v", err)
	}
	if !bytes.Equal(child.Data, raw) {
		t.Errorf("rebased child inline data does not match its manifest")
	}
	if _, err := remote.Image(rebasedName, remote.WithPlatform(amd64)); err != nil {
		t.Errorf("resolving rebased image by platform: %v", err)
	}
	if d := cmp.Diff(append(layerDigests(t, newImg), topDigest), layerDigests(t, img)); d != "" {
		t.Errorf("rebased child layers (-want,+got): %s", d)
	}
}

func TestRebaseIndexDescriptorMetadata(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	newImg, err := random.Image(1024, 1)
	if err != nil {
		t.Fatal(err)
	}
	oldRef, newRef := u.Host+"/base:old", u.Host+"/base:new"
	if err := crane.Push(empty.Image, oldRef); err != nil {
		t.Fatal(err)
	}
	if err := crane.Push(newImg, newRef); err != nil {
		t.Fatal(err)
	}
	oldDigest, err := empty.Image.Digest()
	if err != nil {
		t.Fatal(err)
	}
	newDigest, err := newImg.Digest()
	if err != nil {
		t.Fatal(err)
	}
	platform := &v1.Platform{OS: "linux", Architecture: "amd64"}
	nested := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
		Add: empty.Image, Descriptor: v1.Descriptor{Platform: platform},
	})
	for _, tc := range []struct {
		name  string
		child mutate.Appendable
	}{
		{name: "image", child: empty.Image},
		{name: "nested-index", child: nested},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Two entries share content, but not descriptor metadata.
			adds := make([]mutate.IndexAddendum, 0, 2)
			for _, title := range []string{"first", "second"} {
				adds = append(adds, mutate.IndexAddendum{Add: tc.child, Descriptor: v1.Descriptor{
					Platform: platform,
					Annotations: map[string]string{
						specsv1.AnnotationTitle:           title,
						specsv1.AnnotationBaseImageName:   oldRef,
						specsv1.AnnotationBaseImageDigest: oldDigest.String(),
					},
				}})
			}
			src, err := name.ParseReference(u.Host + "/metadata:" + tc.name)
			if err != nil {
				t.Fatal(err)
			}
			if err := remote.WriteIndex(src, mutate.AppendManifests(empty.Index, adds...)); err != nil {
				t.Fatal(err)
			}
			orig, err := remote.Index(src)
			if err != nil {
				t.Fatal(err)
			}
			rebased, err := rebaseIndex(orig, oldRef, newRef)
			if err != nil {
				t.Fatal(err)
			}
			m, err := rebased.IndexManifest()
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Manifests) != len(adds) {
				t.Fatalf("got %d entries, want %d", len(m.Manifests), len(adds))
			}
			for i, desc := range m.Manifests {
				want := maps.Clone(adds[i].Annotations)
				want[specsv1.AnnotationBaseImageName] = newRef
				want[specsv1.AnnotationBaseImageDigest] = newDigest.String()
				if d := cmp.Diff(want, desc.Annotations); d != "" {
					t.Errorf("entry %d annotations (-want,+got): %s", i, d)
				}
				if d := cmp.Diff(platform, desc.Platform); d != "" {
					t.Errorf("entry %d platform (-want,+got): %s", i, d)
				}
			}
		})
	}
}
