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
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-containerregistry/name"

	"github.com/google/go-containerregistry/v1/random"
)

func TestWrite(t *testing.T) {
	// Make a tempfile for tarball writes.
	fp, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("Error creating temp file.")
	}
	t.Log(fp.Name())
	defer fp.Close()
	defer os.Remove(fp.Name())

	// Make a random image
	randImage, err := random.Image(256, 8)
	if err != nil {
		t.Fatalf("Error creating random image.")
	}
	tag, err := name.NewTag("gcr.io/foo/bar:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error creating test tag.")
	}
	if err := Write(fp.Name(), tag, randImage, nil); err != nil {
		t.Fatalf("Unexpected error writing tarball: %v", err)
	}

	// Make sure the image is valid and can be loaded.
	// Load it both by nil and by its name.
	for _, it := range []*name.Tag{nil, &tag} {
		tarImage, err := Image(fp.Name(), it)
		if err != nil {
			t.Fatalf("Unexpected error reading tarball: %v", err)
		}

		tarManifest, err := tarImage.Manifest()
		if err != nil {
			t.Fatalf("Unexpected error reading tarball: %v", err)
		}
		randManifest, err := randImage.Manifest()
		if err != nil {
			t.Fatalf("Unexpected error reading tarball: %v", err)
		}

		if !reflect.DeepEqual(tarManifest, randManifest) {
			t.Errorf("Manifests not equal. Expected %v\n%v", tarManifest, randManifest)
		}
	}

	// Try loading a different tag, it should error.
	fakeTag, err := name.NewTag("gcr.io/notthistag:latest", name.StrictValidation)
	if err != nil {
		t.Fatalf("Error generating tag: %v", err)
	}
	if _, err := Image(fp.Name(), &fakeTag); err == nil {
		t.Errorf("Expected error loading tag %v from image", fakeTag)
	}
}
