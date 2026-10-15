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

package windows

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func TestWindows(t *testing.T) {
	tarLayer, err := tarball.LayerFromFile("../../pkg/v1/tarball/testdata/content.tar")
	if err != nil {
		t.Fatalf("Unable to create layer from tar file: %v", err)
	}

	win, err := Windows(tarLayer)
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	if _, err := Windows(win); err == nil {
		t.Error("expected an error double-Windowsifying a layer; got nil")
	}

	rc, err := win.Uncompressed()
	if err != nil {
		t.Fatalf("Uncompressed: %v", err)
	}
	defer rc.Close()
	tr := tar.NewReader(rc)
	var sawHives, sawFiles bool
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if h.Name == "Hives" && h.Typeflag == tar.TypeDir {
			sawHives = true
			continue
		}
		if h.Name == "Files" && h.Typeflag == tar.TypeDir {
			sawFiles = true
			continue
		}
		if !strings.HasPrefix(h.Name, "Files/") {
			t.Errorf("tar entry %q didn't have Files prefix", h.Name)
		}
		if h.Format != tar.FormatPAX {
			t.Errorf("tar entry %q had unexpected Format; got %v, want %v", h.Name, h.Format, tar.FormatPAX)
		}
		want := map[string]string{
			"MSWINDOWS.rawsd": userOwnerAndGroupSID,
		}
		if !reflect.DeepEqual(h.PAXRecords, want) {
			t.Errorf("tar entry %q: got %v, want %v", h.Name, h.PAXRecords, want)
		}
	}
	if !sawHives {
		t.Errorf("didn't see Hives/ directory")
	}
	if !sawFiles {
		t.Errorf("didn't see Files/ directory")
	}
}

// layerFromEntry builds an uncompressed single-entry tar layer. link is only
// used for symlink/hardlink types.
func layerFromEntry(t *testing.T, name, link string, typ byte) v1.Layer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{Name: name, Linkname: link, Typeflag: typ, Mode: 0644, Format: tar.FormatPAX}
	if typ == tar.TypeReg {
		hdr.Size = int64(len("hello"))
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if typ == tar.TypeReg {
		if _, err := tw.Write([]byte("hello")); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(raw)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return layer
}

// TestWindowsRejectsTraversal ensures entry names and link targets that climb
// out of the layer root with ".." are rejected rather than re-rooted outside
// the Files/ subtree (zip-slip).
func TestWindowsRejectsTraversal(t *testing.T) {
	for _, tc := range []struct {
		desc, name, link string
		typ              byte
	}{
		{desc: "parent escape", name: "../evil.txt", typ: tar.TypeReg},
		{desc: "nested escape", name: "a/../../escape", typ: tar.TypeReg},
		{desc: "symlink escape", name: "link", link: "../../../etc/passwd", typ: tar.TypeSymlink},
		{desc: "hardlink escape", name: "link", link: "../../../etc/passwd", typ: tar.TypeLink},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			if _, err := Windows(layerFromEntry(t, tc.name, tc.link, tc.typ)); err == nil {
				t.Fatalf("Windows accepted traversal entry %q (link %q); want error", tc.name, tc.link)
			}
		})
	}

	// A normal entry is still converted without error.
	if _, err := Windows(layerFromEntry(t, "bin/sh", "", tar.TypeReg)); err != nil {
		t.Fatalf("Windows rejected a valid entry: %v", err)
	}
}
