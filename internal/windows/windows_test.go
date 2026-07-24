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

func TestWindowsRejectsUnsafePaths(t *testing.T) {
	for _, name := range []string{"../Hives/escaped.txt", "/Hives/escaped.txt"} {
		t.Run(name, func(t *testing.T) {
			layer := layerWithFile(t, name)
			if _, err := Windows(layer); err == nil {
				t.Fatalf("Windows accepted unsafe path %q", name)
			}
		})
	}
}

func layerWithFile(t *testing.T, name string) v1.Layer {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	content := []byte("test")
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     int64(len(content)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return layer
}
