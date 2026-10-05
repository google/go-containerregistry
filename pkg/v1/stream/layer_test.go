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

package stream

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestStreamVsBuffer(t *testing.T) {
	var n int64 = 10000
	newBlob := func() io.ReadCloser { return io.NopCloser(bytes.NewReader(bytes.Repeat([]byte{'a'}, int(n)))) }

	// Test that buffering the same contents and using
	// tarball.LayerFromOpener results in the same digest/diffID/size.
	tl, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) { return newBlob(), nil })
	if err != nil {
		t.Fatalf("LayerFromOpener: %v", err)
	}
	wantDigest, err := tl.Digest()
	if err != nil {
		t.Fatalf("tl.Digest: %v", err)
	}
	wantDiffID, err := tl.DiffID()
	if err != nil {
		t.Fatalf("tl.DiffID: %v", err)
	}
	wantSize, err := tl.Size()
	if err != nil {
		t.Fatalf("tl.Size: %v", err)
	}

	// Check that streaming some content results in the expected digest/diffID/size.
	l := NewLayer(newBlob())
	if c, err := l.Compressed(); err != nil {
		t.Errorf("Compressed: %v", err)
	} else {
		if _, err := io.Copy(io.Discard, c); err != nil {
			t.Errorf("error reading Compressed: %v", err)
		}
		if err := c.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}
	if d, err := l.Digest(); err != nil {
		t.Errorf("Digest: %v", err)
	} else if d != wantDigest {
		t.Errorf("stream Digest got %q, want %q", d, wantDigest)
	}
	if d, err := l.DiffID(); err != nil {
		t.Errorf("DiffID: %v", err)
	} else if d != wantDiffID {
		t.Errorf("stream DiffID got %q, want %q", d, wantDiffID)
	}
	if s, err := l.Size(); err != nil {
		t.Errorf("Size: %v", err)
	} else if s != wantSize {
		t.Errorf("stream Size got %d, want %d", s, wantSize)
	}

	// Test with different compression
	l2 := NewLayer(newBlob(), WithCompressionLevel(2))
	l2WantDigests := []string{
		"sha256:c9afe7b0da6783232e463e12328cb306142548384accf3995806229c9a6a707f",
		"sha256:62e1f590258ecc3b9117fbaf418ac6c4a94ad42102e8b58d47bce9f28a4e9b57", // Go 1.27+
	}
	if c, err := l2.Compressed(); err != nil {
		t.Errorf("Compressed: %v", err)
	} else {
		if _, err := io.Copy(io.Discard, c); err != nil {
			t.Errorf("error reading Compressed: %v", err)
		}
		if err := c.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}
	if d, err := l2.Digest(); err != nil {
		t.Errorf("Digest: %v", err)
	} else if !slices.Contains(l2WantDigests, d.String()) {
		t.Errorf("stream Digest got %q, want one of %v", d.String(), l2WantDigests)
	}
}

func TestLargeStream(t *testing.T) {
	var n int64 = 10000000
	wantSizes := []int64{10000788, 10000785} // Go <=1.26 vs Go 1.27+
	sl := NewLayer(io.NopCloser(io.LimitReader(rand.Reader, n)))
	rc, err := sl.Compressed()
	if err != nil {
		t.Fatalf("Uncompressed: %v", err)
	}
	if _, err := io.Copy(io.Discard, rc); err != nil {
		t.Fatalf("Reading layer: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if dig, err := sl.Digest(); err != nil {
		t.Errorf("Digest: %v", err)
	} else if dig.String() == (v1.Hash{}).String() {
		t.Errorf("Digest got %q, want anything else", (v1.Hash{}).String())
	}
	if diffID, err := sl.DiffID(); err != nil {
		t.Errorf("DiffID: %v", err)
	} else if diffID.String() == (v1.Hash{}).String() {
		t.Errorf("DiffID got %q, want anything else", (v1.Hash{}).String())
	}
	if size, err := sl.Size(); err != nil {
		t.Errorf("Size: %v", err)
	} else if !slices.Contains(wantSizes, size) {
		t.Errorf("Size got %d, want one of %v", size, wantSizes)
	}
}

func TestStreamableLayerFromTarball(t *testing.T) {
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)
	go func() {
		// "Stream" a bunch of files into the layer.
		pw.CloseWithError(func() error {
			for i := 0; i < 1000; i++ {
				name := fmt.Sprintf("file-%d.txt", i)
				body := fmt.Sprintf("i am file number %d", i)
				if err := tw.WriteHeader(&tar.Header{
					Name:     name,
					Mode:     0600,
					Size:     int64(len(body)),
					Typeflag: tar.TypeReg,
				}); err != nil {
					return err
				}
				if _, err := tw.Write([]byte(body)); err != nil {
					return err
				}
			}
			return tw.Close()
		}())
	}()

	l := NewLayer(pr)
	rc, err := l.Compressed()
	if err != nil {
		t.Fatalf("Compressed: %v", err)
	}
	if _, err := io.Copy(io.Discard, rc); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	wantDigests := []string{
		"sha256:ed80efd7e7e884fb59db568f234332283b341b96155e872d638de42d55a34198",
		"sha256:37c500522893ed4f80e732b121c156582d0a5a9a68fbf7eef476ab7f05d58bda", // Go 1.27+
	}
	if got, err := l.Digest(); err != nil {
		t.Errorf("Digest: %v", err)
	} else if !slices.Contains(wantDigests, got.String()) {
		t.Errorf("Digest: got %q, want one of %v", got, wantDigests)
	}
}

// TestNotComputed tests that Digest/DiffID/Size return ErrNotComputed before
// the stream has been consumed.
func TestNotComputed(t *testing.T) {
	l := NewLayer(io.NopCloser(bytes.NewBufferString("hi")))

	// All methods should return ErrNotComputed until the stream has been
	// consumed and closed.
	if _, err := l.Size(); !errors.Is(err, ErrNotComputed) {
		t.Errorf("Size: got %v, want %v", err, ErrNotComputed)
	}
	if _, err := l.Digest(); err == nil {
		t.Errorf("Digest: got %v, want %v", err, ErrNotComputed)
	}
	if _, err := l.DiffID(); err == nil {
		t.Errorf("DiffID: got %v, want %v", err, ErrNotComputed)
	}
}

// TestConsumed tests that Compressed returns ErrConsumed when the stream has
// already been consumed.
func TestConsumed(t *testing.T) {
	l := NewLayer(io.NopCloser(strings.NewReader("hello")))
	rc, err := l.Compressed()
	if err != nil {
		t.Errorf("Compressed: %v", err)
	}
	if _, err := io.Copy(io.Discard, rc); err != nil {
		t.Errorf("Error reading contents: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	if _, err := l.Compressed(); !errors.Is(err, ErrConsumed) {
		t.Errorf("Compressed() after consuming; got %v, want %v", err, ErrConsumed)
	}
}

func TestCloseTextStreamBeforeConsume(t *testing.T) {
	// Create stream layer from tar pipe
	l := NewLayer(io.NopCloser(strings.NewReader("hello")))
	rc, err := l.Compressed()
	if err != nil {
		t.Fatalf("Compressed: %v", err)
	}

	// Close stream layer before consuming
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestCloseTarStreamBeforeConsume(t *testing.T) {
	// Write small tar to pipe
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)
	go func() {
		pw.CloseWithError(func() error {
			body := "test file"
			if err := tw.WriteHeader(&tar.Header{
				Name:     "test.txt",
				Mode:     0600,
				Size:     int64(len(body)),
				Typeflag: tar.TypeReg,
			}); err != nil {
				return err
			}
			if _, err := tw.Write([]byte(body)); err != nil {
				return err
			}
			return tw.Close()
		}())
	}()

	// Create stream layer from tar pipe
	l := NewLayer(pr)
	rc, err := l.Compressed()
	if err != nil {
		t.Fatalf("Compressed: %v", err)
	}

	// Close stream layer before consuming
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestMediaType(t *testing.T) {
	l := NewLayer(io.NopCloser(strings.NewReader("hello")))
	mediaType, err := l.MediaType()

	if err != nil {
		t.Fatalf("MediaType(): %v", err)
	}

	if got, want := mediaType, types.DockerLayer; got != want {
		t.Errorf("MediaType(): want %q, got %q", want, got)
	}
}

func TestMediaTypeOption(t *testing.T) {
	l := NewLayer(io.NopCloser(strings.NewReader("hello")), WithMediaType(types.OCILayer))
	mediaType, err := l.MediaType()

	if err != nil {
		t.Fatalf("MediaType(): %v", err)
	}

	if got, want := mediaType, types.OCILayer; got != want {
		t.Errorf("MediaType(): want %q, got %q", want, got)
	}
}
