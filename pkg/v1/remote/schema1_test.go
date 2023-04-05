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

package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

var fatal = log.Fatal
var helper = func() {}

func must[T any](t T, err error) T {
	helper()
	if err != nil {
		fatal(err)
	}
	return t
}

type fakeSchema1 struct {
	b []byte
}

func (f *fakeSchema1) MediaType() (types.MediaType, error) {
	return types.DockerManifestSchema1, nil
}

func (f *fakeSchema1) RawManifest() ([]byte, error) {
	return f.b, nil
}

func toSchema1(t *testing.T, img v1.Image) *fakeSchema1 {
	t.Helper()

	fsl := []fslayer{}

	layers := must(img.Layers())
	for i := len(layers) - 1; i >= 0; i-- {
		l := layers[i]
		dig := must(l.Digest())
		fsl = append(fsl, fslayer{
			BlobSum: dig.String(),
		})
	}

	return &fakeSchema1{
		b: must(json.Marshal(&schema1Manifest{FSLayers: fsl})),
	}
}

func TestSchema1(t *testing.T) {
	fatal = t.Fatal
	helper = t.Helper

	rnd := must(random.Image(1024, 3))
	s1 := toSchema1(t, rnd)

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u := must(url.Parse(s.URL))

	dst := fmt.Sprintf("%s/test/foreign/upload", u.Host)
	ref := must(name.ParseReference(dst))

	if err := Write(ref, rnd); err != nil {
		t.Fatal(err)
	}

	tag := ref.Context().Tag("schema1")

	if err := Put(tag, s1); err != nil {
		t.Fatal(err)
	}

	pulled := must(Get(tag))
	img := must(pulled.Schema1())

	if err := Write(ref.Context().Tag("repushed"), img); err != nil {
		t.Fatal(err)
	}

	mustErr := func(a any, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("should have failed, got %T", a)
		}
	}

	mustErr(img.ConfigFile())
	mustErr(img.Manifest())
	mustErr(img.LayerByDiffID(v1.Hash{}))

	h, sz, err := v1.SHA256(bytes.NewReader(s1.b))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := must(img.Size()), sz; got != want {
		t.Errorf("Size(): got %d, want %d", got, want)
	}
	if got, want := must(img.Digest()), h; got != want {
		t.Errorf("Digest(): got %s, want %s", got, want)
	}

	if got, want := must(io.ReadAll(mutate.Extract(img))), must(io.ReadAll(mutate.Extract(rnd))); !bytes.Equal(got, want) {
		t.Error("filesystems are different")
	}
}
