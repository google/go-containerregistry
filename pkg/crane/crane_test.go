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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// TODO(jonjohnsonjr): Test crane.Delete behavior.
// TODO(jonjohnsonjr): Test crane.ListTags behavior.
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
	manifest, err := img.RawManifest()
	if err != nil {
		t.Fatal(err)
	}
	config, err := img.RawConfigFile()
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
	} else if string(m) != string(manifest) {
		t.Errorf("Manifest(): %v != %v", m, manifest)
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
	} else if m, err := pulled.RawManifest(); err != nil {
		t.Fatal(err)
	} else if string(m) != string(manifest) {
		t.Errorf("crane.Pull().Manifest(): %v != %v", m, manifest)
	}

	// Test that the copied image is the same as the source.
	if err := crane.Copy(src, dst); err != nil {
		t.Fatal(err)
	}

	if _, err := crane.Pull(dst); err != nil {
		t.Fatal(err)
	}

	d, err = crane.Digest(dst)
	if err != nil {
		t.Fatal(err)
	} else if d != digest.String() {
		t.Errorf("Copied Digest(): %v != %v", d, digest)
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

func TestCraneTarball(t *testing.T) {
	t.Parallel()
	// Write an image as a tarball.
	tmp, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
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

func TestCraneFilesystem(t *testing.T) {
	t.Parallel()
	tmp, err := ioutil.TempFile("", "")
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
		if err == io.EOF {
			t.Fatalf("didn't find find")
		} else if err != nil {
			t.Fatal(err)
		}
		if header.Name == name {
			b, err := ioutil.ReadAll(tr)
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

func TestBadInputs(t *testing.T) {
	t.Parallel()
	invalid := "@@@@@@"

	// Create a valid image reference that will fail with not found.
	s := httptest.NewServer(http.NotFoundHandler())
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	valid := fmt.Sprintf("%s/some/image", u.Host)

	// e drops the first parameter so we can use the result of a function
	// that returns two values as an expression above. This is a bit of a go quirk.
	e := func(_ interface{}, err error) error {
		return err
	}

	for _, err := range []error{
		crane.Push(nil, invalid),
		crane.Delete(invalid),
		crane.Delete(valid), // 404
		crane.Save(nil, invalid, ""),
		crane.Copy(invalid, invalid),
		crane.Copy(valid, invalid),
		crane.Copy(valid, valid), // 404
		// These return multiple values, which are hard to use as expressions.
		e(crane.Pull(invalid)),
		e(crane.Digest(invalid)),
		e(crane.Manifest(invalid)),
		e(crane.Config(invalid)),
		e(crane.Config(valid)), // 404
		e(crane.ListTags(invalid)),
		e(crane.ListTags(valid)), // 404
		e(crane.Append(nil, invalid)),
	} {
		if err == nil {
			t.Error("expected err, got nil")
		}
	}
}
