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
)

// An empty annotation value is explicitly allowed by the image spec, and an
// empty label value is the config-level equivalent.
func TestMutateEmptyLabelAndAnnotationValues(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	src := fmt.Sprintf("%s/test/mutate:latest", u.Host)

	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	if err := crane.Push(img, src); err != nil {
		t.Fatalf("crane.Push: %v", err)
	}

	options := []crane.Option{}
	cmd := NewCmdMutate(&options)
	cmd.SetArgs([]string{src, "--label", "empty.label=", "--annotation", "empty.annotation="})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mutate with empty values: %v", err)
	}

	pulled, err := crane.Pull(src)
	if err != nil {
		t.Fatalf("crane.Pull: %v", err)
	}

	cfg, err := pulled.ConfigFile()
	if err != nil {
		t.Fatalf("ConfigFile: %v", err)
	}
	got, ok := cfg.Config.Labels["empty.label"]
	if !ok {
		t.Errorf("label %q was not set at all", "empty.label")
	} else if got != "" {
		t.Errorf("label %q = %q, want empty string", "empty.label", got)
	}

	manifest, err := pulled.Manifest()
	if err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	got, ok = manifest.Annotations["empty.annotation"]
	if !ok {
		t.Errorf("annotation %q was not set at all", "empty.annotation")
	} else if got != "" {
		t.Errorf("annotation %q = %q, want empty string", "empty.annotation", got)
	}
}

// TestMutateIndexAnnotations covers crane mutate on an index reference:
// annotations edit the index in place, -t writes the result to a new tag,
// image-only flags are rejected, and --platform mutates the platform image.
func TestMutateIndexAnnotations(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	amd64Img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	arm64Img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image: %v", err)
	}
	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: amd64Img, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}}},
		mutate.IndexAddendum{Add: arm64Img, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}}},
	)
	idx = mutate.Annotations(idx, map[string]string{"foo": "old", "keep": "original"}).(v1.ImageIndex)
	origManifest, err := idx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest: %v", err)
	}

	src := fmt.Sprintf("%s/test/mutate-index:latest", u.Host)
	srcRef, err := name.ParseReference(src)
	if err != nil {
		t.Fatalf("name.ParseReference: %v", err)
	}
	if err := remote.WriteIndex(srcRef, idx); err != nil {
		t.Fatalf("remote.WriteIndex: %v", err)
	}

	options := []crane.Option{}

	// Without --platform, annotations are applied to the index itself and the
	// child manifests are kept as they were.
	annotate := NewCmdMutate(&options)
	annotate.SetArgs([]string{src, "--annotation", "foo=bar", "--annotation", "empty="})
	annotate.SetOut(io.Discard)
	annotate.SetErr(io.Discard)
	if err := annotate.Execute(); err != nil {
		t.Fatalf("mutate index annotations: %v", err)
	}
	gotIdx, err := remote.Index(srcRef)
	if err != nil {
		t.Fatalf("remote.Index: %v", err)
	}
	gotManifest, err := gotIdx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest: %v", err)
	}
	if got := gotManifest.Annotations["foo"]; got != "bar" {
		t.Errorf("annotation foo = %q, want %q", got, "bar")
	}
	if got, ok := gotManifest.Annotations["empty"]; !ok || got != "" {
		t.Errorf("annotation empty = %q (set=%t), want an empty value that is set", got, ok)
	}
	if got := gotManifest.Annotations["keep"]; got != "original" {
		t.Errorf("existing annotation keep = %q, want original", got)
	}
	if d := cmp.Diff(origManifest.Manifests, gotManifest.Manifests); d != "" {
		t.Errorf("child manifests changed (-want,+got): %s", d)
	}

	// -t writes the mutated index to the new tag, prints its digest reference,
	// and leaves the source tag untouched.
	tagged := fmt.Sprintf("%s/test/mutate-index:tagged", u.Host)
	taggedRef, err := name.ParseReference(tagged)
	if err != nil {
		t.Fatalf("name.ParseReference: %v", err)
	}
	var out bytes.Buffer
	tag := NewCmdMutate(&options)
	tag.SetArgs([]string{src, "--annotation", "tagged=yes", "-t", tagged})
	tag.SetOut(&out)
	tag.SetErr(io.Discard)
	if err := tag.Execute(); err != nil {
		t.Fatalf("mutate index with -t: %v", err)
	}
	taggedIdx, err := remote.Index(taggedRef)
	if err != nil {
		t.Fatalf("remote.Index(%s): %v", tagged, err)
	}
	taggedManifest, err := taggedIdx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest: %v", err)
	}
	if got := taggedManifest.Annotations["tagged"]; got != "yes" {
		t.Errorf("tagged annotation = %q, want %q", got, "yes")
	}
	taggedDigest, err := taggedIdx.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if want, got := taggedRef.Context().Digest(taggedDigest.String()).String(), strings.TrimSpace(out.String()); got != want {
		t.Errorf("printed digest reference = %q, want %q", got, want)
	}
	srcAfter, err := remote.Index(srcRef)
	if err != nil {
		t.Fatalf("remote.Index: %v", err)
	}
	srcAfterManifest, err := srcAfter.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest: %v", err)
	}
	if _, ok := srcAfterManifest.Annotations["tagged"]; ok {
		t.Errorf("source tag gained the -t annotation; it should be left alone")
	}

	// Digest inputs and --repo push by the new digest, without moving the source tag.
	sourceDigest, err := srcAfter.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	for _, tc := range []struct {
		name   string
		source string
		repo   string
	}{
		{name: "digest", source: srcRef.Context().Digest(sourceDigest.String()).String()},
		{name: "repository", source: src, repo: u.Host + "/test/mutate-destination"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{tc.source, "-a", "destination=" + tc.name}
			wantRepo := srcRef.Context().Name()
			if tc.repo != "" {
				args = append(args, "--repo", tc.repo)
				wantRepo = tc.repo
			}
			var out bytes.Buffer
			command := NewCmdMutate(&options)
			command.SetArgs(args)
			command.SetOut(&out)
			command.SetErr(io.Discard)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			dest, err := name.NewDigest(strings.TrimSpace(out.String()))
			if err != nil {
				t.Fatal(err)
			}
			if dest.Context().Name() != wantRepo || dest.DigestStr() == sourceDigest.String() {
				t.Fatalf("destination = %s, want a new digest in %s", dest, wantRepo)
			}
			idx, err := remote.Index(dest)
			if err != nil {
				t.Fatal(err)
			}
			m, err := idx.IndexManifest()
			if err != nil {
				t.Fatal(err)
			}
			if got := m.Annotations["destination"]; got != tc.name {
				t.Errorf("destination annotation = %q, want %q", got, tc.name)
			}
			if d := cmp.Diff(origManifest.Manifests, m.Manifests); d != "" {
				t.Errorf("child manifests changed (-want,+got): %s", d)
			}
			head, err := crane.Head(src)
			if err != nil {
				t.Fatal(err)
			}
			if head.Digest != sourceDigest {
				t.Errorf("source tag moved to %s, want %s", head.Digest, sourceDigest)
			}
		})
	}

	// Image-only flags and tarball output have no meaning for an index without --platform.
	for _, args := range [][]string{{"--label", "l=v"}, {"--env", "E=V"}, {"--output", "index.tar"}} {
		reject := NewCmdMutate(&options)
		reject.SetArgs(append([]string{src}, args...))
		reject.SetOut(io.Discard)
		reject.SetErr(io.Discard)
		if err := reject.Execute(); err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Errorf("mutate %v on index without --platform: err = %v, want not supported", args, err)
		}
	}

	// With --platform, the platform-specific image is mutated: its config and
	// manifest change, and its layers are the amd64 child's layers.
	platOpts := []crane.Option{crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"})}
	platformRef := fmt.Sprintf("%s/test/mutate-index:amd64", u.Host)
	platform := NewCmdMutate(&platOpts)
	platform.SetArgs([]string{src, "--label", "l=v", "--annotation", "a=b", "-t", platformRef})
	platform.SetOut(io.Discard)
	platform.SetErr(io.Discard)
	if err := platform.Execute(); err != nil {
		t.Fatalf("mutate --platform linux/amd64: %v", err)
	}
	platRef, err := name.ParseReference(platformRef)
	if err != nil {
		t.Fatalf("name.ParseReference: %v", err)
	}
	pd, err := remote.Get(platRef)
	if err != nil {
		t.Fatalf("remote.Get: %v", err)
	}
	if !pd.MediaType.IsImage() {
		t.Fatalf("%s is %s, want an image", platformRef, pd.MediaType)
	}
	pimg, err := pd.Image()
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	pcfg, err := pimg.ConfigFile()
	if err != nil {
		t.Fatalf("ConfigFile: %v", err)
	}
	if got := pcfg.Config.Labels["l"]; got != "v" {
		t.Errorf("label l = %q, want %q", got, "v")
	}
	pm, err := pimg.Manifest()
	if err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	if got := pm.Annotations["a"]; got != "b" {
		t.Errorf("annotation a = %q, want %q", got, "b")
	}
	wantLayers, err := amd64Img.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}
	gotLayers, err := pimg.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}
	if len(gotLayers) != len(wantLayers) {
		t.Fatalf("mutated image has %d layers, want %d", len(gotLayers), len(wantLayers))
	}
	for i := range wantLayers {
		want, err := wantLayers[i].Digest()
		if err != nil {
			t.Fatalf("Digest: %v", err)
		}
		got, err := gotLayers[i].Digest()
		if err != nil {
			t.Fatalf("Digest: %v", err)
		}
		if got != want {
			t.Errorf("layer %d = %s, want %s from the amd64 child", i, got, want)
		}
	}
}
