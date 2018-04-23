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

package tarball

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/name"
)

func TestFlattenWhiteout(t *testing.T) {
	img, err := ImageFromPath("whiteout_image.tar", nil)
	if err != nil {
		t.Errorf("Error loading image: %v", err)
	}
	tarPath, _ := filepath.Abs("img.tar")
	defer os.Remove(tarPath)
	if err := Flatten(img, tarPath); err != nil {
		t.Errorf("Error when flattening image: %v", err)
	}
	f, err := os.Open(tarPath)
	if err != nil {
		t.Errorf("Error when opening tar file for reading: %v", err)
	}
	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		name := header.Name
		for _, part := range filepath.SplitList(name) {
			if part == "foo" {
				t.Errorf("whiteout file found in tar: %v", name)
			}
		}
	}
}

func TestFlattenOverwrittenFile(t *testing.T) {
	img, err := ImageFromPath("overwritten_file.tar", nil)
	if err != nil {
		t.Fatalf("Error loading image: %v", err)
	}
	tarPath, _ := filepath.Abs("img.tar")
	defer os.Remove(tarPath)
	if err := Flatten(img, tarPath); err != nil {
		t.Errorf("Error when flattening image: %v", err)
	}
	f, err := os.Open(tarPath)
	if err != nil {
		t.Errorf("Error when opening tar file for reading: %v", err)
	}
	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		name := header.Name
		if strings.Contains(name, "foo.txt") {
			buf := new(bytes.Buffer)
			buf.ReadFrom(tr)
			if strings.Contains(buf.String(), "foo") {
				t.Errorf("Contents of file were not correctly overwritten")
			}
		}
	}
}

func TestWhiteoutDir(t *testing.T) {
	fsMap := map[string]bool{
		"baz":      true,
		"red/blue": true,
	}
	var tests = []struct {
		path     string
		whiteout bool
	}{
		{"usr/bin", false},
		{"baz/foo.txt", true},
		{"baz/bar/foo.txt", true},
		{"red/green", false},
		{"red/yellow.txt", false},
	}

	for _, tt := range tests {
		whiteout := InWhiteoutDir(fsMap, tt.path)
		if whiteout != tt.whiteout {
			t.Errorf("Whiteout %s: expected %v, but got %v", tt.path, tt.whiteout, whiteout)
		}
	}
}

func TestManifestAndConfig(t *testing.T) {
	img, err := ImageFromPath("test_image_1.tar", nil)
	if err != nil {
		t.Fatalf("Error loading image: %v", err)
	}
	manifest, err := img.Manifest()
	if err != nil {
		t.Fatalf("Error loading manifest: %v", err)
	}
	if len(manifest.Layers) != 1 {
		t.Fatalf("layers should be 1, got %d", len(manifest.Layers))
	}

	config, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("Error loading config file: %v", err)
	}
	if len(config.History) != 1 {
		t.Fatalf("history length should be 1, got %d", len(config.History))
	}
}

func TestNoManifest(t *testing.T) {
	img, err := ImageFromPath("testdata/no_manifest.tar", nil)
	if err == nil {
		t.Fatalf("Error expected loading image: %v", img)
	}
}

func TestBundleSingle(t *testing.T) {
	img, err := ImageFromPath("test_bundle.tar", nil)
	if err == nil {
		t.Fatalf("Error expected loading image: %v", img)
	}
}

func TestBundleMultiple(t *testing.T) {
	for _, imgName := range []string{
		"test_image_1",
		"test_image_2",
		"test_image_1:latest",
		"test_image_2:latest",
		"index.docker.io/library/test_image_1:latest",
	} {
		t.Run(imgName, func(t *testing.T) {
			tag, err := name.NewTag(imgName, name.WeakValidation)
			if err != nil {
				t.Fatalf("Error creating tag: %v", err)
			}
			img, err := ImageFromPath("test_bundle.tar", &tag)
			if err != nil {
				t.Fatalf("Error loading image: %v", err)
			}
			if _, err := img.Manifest(); err != nil {
				t.Fatalf("Unexpected error loading manifest: %v", err)
			}
		})
	}
}
