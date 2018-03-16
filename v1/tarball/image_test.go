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
	"path/filepath"
	"testing"
)

var testdata = "testdata"

func TestManifestAndConfig(t *testing.T) {
	img, err := Image(filepath.Join(testdata, "test_image.tar"))
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
	img, err := Image(filepath.Join(testdata, "no_manifest.tar"))
	if err != nil {
		t.Fatalf("Error loading image: %v", err)
	}
	if _, err := img.Manifest(); err == nil {
		t.Fatalf("Error expected loading manifest.")
	}
}

func TestBundle(t *testing.T) {
	img, err := Image(filepath.Join(testdata, "no_manifest.tar"))
	if err != nil {
		t.Fatalf("Error loading image: %v", err)
	}
	if _, err := img.Manifest(); err == nil {
		t.Fatalf("Error expected loading manifest.")
	}
}
