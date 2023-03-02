// Copyright 2019 Google LLC All Rights Reserved.
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
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/internal/compare"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// TODO(jonjohnsonjr): Test crane.Copy failures.
func TestCraneRegistry(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	src := fmt.Sprintf("%s/test/crane", u.Host)
	dst := fmt.Sprintf("%s/test/crane/copy", u.Host)

	// Expected values.
	img, err := random.Image(1024, 5)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}
	rawManifest, err := img.RawManifest()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := img.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	config, err := img.RawConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	layer, err := img.LayerByDigest(manifest.Layers[0].Digest)
	if err != nil {
		t.Fatal(err)
	}

	// Load up the registry.
	if err := crane.Push(img, src); err != nil {
		t.Fatal(err)
	}

	// Test that `crane.Foo` returns expected values.
	d, err := crane.Digest(src)
	if err != nil {
		t.Error(err)
	} else if d != digest.String() {
		t.Errorf("Digest(): %v != %v", d, digest)
	}

	m, err := crane.Manifest(src)
	if err != nil {
		t.Error(err)
	} else if string(m) != string(rawManifest) {
		t.Errorf("Manifest(): %v != %v", m, rawManifest)
	}

	c, err := crane.Config(src)
	if err != nil {
		t.Error(err)
	} else if string(c) != string(config) {
		t.Errorf("Config(): %v != %v", c, config)
	}

	// Make sure we pull what we pushed.
	pulled, err := crane.Pull(src)
	if err != nil {
		t.Error(err)
	}
	if err := compare.Images(img, pulled); err != nil {
		t.Fatal(err)
	}

	// Test that the copied image is the same as the source.
	if err := crane.Copy(src, dst); err != nil {
		t.Fatal(err)
	}

	// Make sure what we copied is equivalent.
	// Also, get options coverage in a dumb way.
	copied, err := crane.Pull(dst, crane.Insecure, crane.WithTransport(http.DefaultTransport), crane.WithAuth(authn.Anonymous), crane.WithAuthFromKeychain(authn.DefaultKeychain), crane.WithUserAgent("crane/tests"))
	if err != nil {
		t.Fatal(err)
	}
	if err := compare.Images(pulled, copied); err != nil {
		t.Fatal(err)
	}

	if err := crane.Tag(dst, "crane-tag"); err != nil {
		t.Fatal(err)
	}

	// Make sure what we tagged is equivalent.
	tagged, err := crane.Pull(fmt.Sprintf("%s:%s", dst, "crane-tag"))
	if err != nil {
		t.Fatal(err)
	}
	if err := compare.Images(pulled, tagged); err != nil {
		t.Fatal(err)
	}

	layerRef := fmt.Sprintf("%s/test/crane@%s", u.Host, manifest.Layers[0].Digest)
	pulledLayer, err := crane.PullLayer(layerRef)
	if err != nil {
		t.Fatal(err)
	}

	if err := compare.Layers(pulledLayer, layer); err != nil {
		t.Fatal(err)
	}

	// List Tags
	// dst variable have: latest and crane-tag
	tags, err := crane.ListTags(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Fatalf("wanted 2 tags, got %d", len(tags))
	}

	// create 4 tags for dst
	for i := 1; i < 5; i++ {
		if err := crane.Tag(dst, fmt.Sprintf("honk-tag-%d", i)); err != nil {
			t.Fatal(err)
		}
	}

	tags, err = crane.ListTags(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 6 {
		t.Fatalf("wanted 6 tags, got %d", len(tags))
	}

	// Delete the non existing image
	if err := crane.Delete(dst + ":honk-image"); err == nil {
		t.Fatal("wanted err, got nil")
	}

	// Delete the image
	if err := crane.Delete(src); err != nil {
		t.Fatal(err)
	}

	// check if the image was really deleted
	if _, err := crane.Pull(src); err == nil {
		t.Fatal("wanted err, got nil")
	}

	// check if the copied image still exist
	dstPulled, err := crane.Pull(dst)
	if err != nil {
		t.Fatal(err)
	}
	if err := compare.Images(dstPulled, copied); err != nil {
		t.Fatal(err)
	}

	// List Catalog
	repos, err := crane.Catalog(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("wanted 2 repos, got %d", len(repos))
	}

	// Test pushing layer
	layer, err = img.LayerByDigest(manifest.Layers[1].Digest)
	if err != nil {
		t.Fatal(err)
	}
	if err := crane.Upload(layer, dst); err != nil {
		t.Fatal(err)
	}
}

func TestCraneCopyIndex(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	src := fmt.Sprintf("%s/test/crane", u.Host)
	dst := fmt.Sprintf("%s/test/crane/copy", u.Host)

	// Load up the registry.
	idx, err := random.Index(1024, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := name.ParseReference(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.WriteIndex(ref, idx); err != nil {
		t.Fatal(err)
	}

	// Test that the copied index is the same as the source.
	if err := crane.Copy(src, dst); err != nil {
		t.Fatal(err)
	}

	d, err := crane.Digest(src)
	if err != nil {
		t.Fatal(err)
	}
	cp, err := crane.Digest(dst)
	if err != nil {
		t.Fatal(err)
	}
	if d != cp {
		t.Errorf("Copied Digest(): %v != %v", d, cp)
	}
}

func TestWithPlatform(t *testing.T) {
	// Set up a fake registry with a platform-specific image.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	imgs := []mutate.IndexAddendum{}
	for _, plat := range []string{
		"linux/amd64",
		"linux/arm",
	} {
		img, err := crane.Image(map[string][]byte{
			"platform.txt": []byte(plat),
		})
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(plat, "/")
		imgs = append(imgs, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{
					OS:           parts[0],
					Architecture: parts[1],
				},
			},
		})
	}

	idx := mutate.AppendManifests(empty.Index, imgs...)

	src := path.Join(u.Host, "src")
	dst := path.Join(u.Host, "dst")

	ref, err := name.ParseReference(src)
	if err != nil {
		t.Fatal(err)
	}

	// Populate registry so we can copy from it.
	if err := remote.WriteIndex(ref, idx); err != nil {
		t.Fatal(err)
	}

	if err := crane.Copy(src, dst, crane.WithPlatform(imgs[1].Platform)); err != nil {
		t.Fatal(err)
	}

	want, err := crane.Manifest(src, crane.WithPlatform(imgs[1].Platform))
	if err != nil {
		t.Fatal(err)
	}
	got, err := crane.Manifest(dst)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != string(want) {
		t.Errorf("Manifest(%q) != Manifest(%q): (\n\n%s\n\n!=\n\n%s\n\n)", dst, src, string(got), string(want))
	}

	arch := "real fake doors"

	// Now do a fake platform, should fail
	if _, err := crane.Manifest(src, crane.WithPlatform(&v1.Platform{
		OS:           "does-not-exist",
		Architecture: arch,
	})); err == nil {
		t.Error("crane.Manifest(fake platform): got nil want err")
	} else if !strings.Contains(err.Error(), arch) {
		t.Errorf("crane.Manifest(fake platform): expected %q in error, got: %v", arch, err)
	}
}

func TestCraneTarball(t *testing.T) {
	t.Parallel()
	// Write an image as a tarball.
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	img, err := random.Image(1024, 5)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}
	src := fmt.Sprintf("test/crane@%s", digest)

	if err := crane.Save(img, src, tmp.Name()); err != nil {
		t.Errorf("Save: %v", err)
	}

	// Make sure the image we load has a matching digest.
	img, err = crane.Load(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}

	d, err := img.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if d != digest {
		t.Errorf("digest mismatch: %v != %v", d, digest)
	}
}

func TestCraneSaveLegacy(t *testing.T) {
	t.Parallel()
	// Write an image as a legacy tarball.
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	img, err := random.Image(1024, 5)
	if err != nil {
		t.Fatal(err)
	}

	if err := crane.SaveLegacy(img, "test/crane", tmp.Name()); err != nil {
		t.Errorf("SaveOCI: %v", err)
	}
}

func TestCraneSaveOCI(t *testing.T) {
	t.Parallel()
	// Write an image as an OCI image layout.
	tmp := t.TempDir()

	img, err := random.Image(1024, 5)
	if err != nil {
		t.Fatal(err)
	}
	if err := crane.SaveOCI(img, tmp); err != nil {
		t.Errorf("SaveLegacy: %v", err)
	}
}

func TestCraneFilesystem(t *testing.T) {
	t.Parallel()
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	img, err := random.Image(1024, 5)
	if err != nil {
		t.Fatal(err)
	}

	name := "/some/file"
	content := []byte("sentinel")

	tw := tar.NewWriter(tmp)
	if err := tw.WriteHeader(&tar.Header{
		Size: int64(len(content)),
		Name: name,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Flush()
	tw.Close()

	img, err = crane.Append(img, tmp.Name())
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := crane.Export(img, &buf); err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(&buf)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			t.Fatalf("didn't find find")
		} else if err != nil {
			t.Fatal(err)
		}
		if header.Name == name {
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != string(content) {
				t.Fatalf("got back wrong content: %v != %v", string(b), string(content))
			}
			break
		}
	}
}

func TestStreamingAppend(t *testing.T) {
	// Stdin will be an uncompressed layer.
	layer, err := crane.Layer(map[string][]byte{
		"hello": []byte(`world`),
	})
	if err != nil {
		t.Fatal(err)
	}
	rc, err := layer.Uncompressed()
	if err != nil {
		t.Fatal(err)
	}

	tmp, err := os.CreateTemp("", "crane-append")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, rc); err != nil {
		t.Fatal(err)
	}

	stdin := os.Stdin
	defer func() {
		os.Stdin = stdin
	}()

	os.Stdin = tmp

	img, err := crane.Append(empty.Image, "-")
	if err != nil {
		t.Fatal(err)
	}
	ll, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if want, got := 1, len(ll); want != got {
		t.Errorf("crane.Append(stdin) - len(layers): want %d != got %d", want, got)
	}
}

func TestBadInputs(t *testing.T) {
	t.Parallel()
	invalid := "/dev/null/@@@@@@"

	// Create a valid image reference that will fail with not found.
	s := httptest.NewServer(http.NotFoundHandler())
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	valid404 := fmt.Sprintf("%s/some/image", u.Host)

	// e drops the first parameter so we can use the result of a function
	// that returns two values as an expression above. This is a bit of a go quirk.
	e := func(_ any, err error) error {
		return err
	}

	for _, tc := range []struct {
		desc string
		err  error
	}{
		{"Push(_, invalid)", crane.Push(nil, invalid)},
		{"Upload(_, invalid)", crane.Upload(nil, invalid)},
		{"Delete(invalid)", crane.Delete(invalid)},
		{"Delete: 404", crane.Delete(valid404)},
		{"Save(_, invalid)", crane.Save(nil, invalid, "")},
		{"SaveLegacy(_, invalid)", crane.SaveLegacy(nil, invalid, "")},
		{"SaveLegacy(_, invalid)", crane.SaveLegacy(nil, valid404, invalid)},
		{"SaveOCI(_, invalid)", crane.SaveOCI(nil, "")},
		{"Copy(invalid, invalid)", crane.Copy(invalid, invalid)},
		{"Copy(404, invalid)", crane.Copy(valid404, invalid)},
		{"Copy(404, 404)", crane.Copy(valid404, valid404)},
		{"Tag(invalid, invalid)", crane.Tag(invalid, invalid)},
		{"Tag(404, invalid)", crane.Tag(valid404, invalid)},
		{"Tag(404, 404)", crane.Tag(valid404, valid404)},
		{"Optimize(invalid, invalid)", crane.Optimize(invalid, invalid, []string{})},
		{"Optimize(404, invalid)", crane.Optimize(valid404, invalid, []string{})},
		{"Optimize(404, 404)", crane.Optimize(valid404, valid404, []string{})},
		// These return multiple values, which are hard to use as expressions.
		{"Pull(invalid)", e(crane.Pull(invalid))},
		{"Digest(invalid)", e(crane.Digest(invalid))},
		{"Manifest(invalid)", e(crane.Manifest(invalid))},
		{"Config(invalid)", e(crane.Config(invalid))},
		{"Config(404)", e(crane.Config(valid404))},
		{"ListTags(invalid)", e(crane.ListTags(invalid))},
		{"ListTags(404)", e(crane.ListTags(valid404))},
		{"Append(_, invalid)", e(crane.Append(nil, invalid))},
		{"Catalog(invalid)", e(crane.Catalog(invalid))},
		{"Catalog(404)", e(crane.Catalog(u.Host))},
		{"PullLayer(invalid)", e(crane.PullLayer(invalid))},
		{"LoadTag(_, invalid)", e(crane.LoadTag("", invalid))},
		{"LoadTag(invalid, 404)", e(crane.LoadTag(invalid, valid404))},
	} {
		if tc.err == nil {
			t.Errorf("%s: expected err, got nil", tc.desc)
		}
	}
}
