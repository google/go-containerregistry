// Copyright 2021 Google LLC All Rights Reserved.
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

package crane

import (
	"bytes"
	"io"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestExport(t *testing.T) {
	want := []byte(`{"foo":"bar"}`)
	layer := static.NewLayer(want, types.MediaType("application/json"))
	img, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Export(img, &buf); err != nil {
		t.Fatal(err)
	}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("got: %s\nwant: %s", got, want)
	}
}

func TestExportClosesSingleLayerReader(t *testing.T) {
	var closed bool
	img := singleLayerImage{layer: closeTrackingLayer{closed: &closed}}

	var buf bytes.Buffer
	if err := Export(img, &buf); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "contents"; got != want {
		t.Fatalf("Export() = %q, want %q", got, want)
	}
	if !closed {
		t.Fatal("Export() did not close the layer reader")
	}
}

type singleLayerImage struct {
	v1.Image
	layer v1.Layer
}

func (i singleLayerImage) Layers() ([]v1.Layer, error) {
	return []v1.Layer{i.layer}, nil
}

type closeTrackingLayer struct {
	closed *bool
}

func (l closeTrackingLayer) Digest() (v1.Hash, error) {
	return v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("0", 64)}, nil
}

func (l closeTrackingLayer) DiffID() (v1.Hash, error) {
	return v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("1", 64)}, nil
}

func (l closeTrackingLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("contents")), nil
}

func (l closeTrackingLayer) MediaType() (types.MediaType, error) {
	return types.MediaType("application/json"), nil
}

func (l closeTrackingLayer) Size() (int64, error) {
	return int64(len("contents")), nil
}

func (l closeTrackingLayer) Uncompressed() (io.ReadCloser, error) {
	return &closeTrackingReadCloser{
		Reader: strings.NewReader("contents"),
		closed: l.closed,
	}, nil
}

type closeTrackingReadCloser struct {
	io.Reader
	closed *bool
}

func (rc *closeTrackingReadCloser) Close() error {
	*rc.closed = true
	return nil
}
