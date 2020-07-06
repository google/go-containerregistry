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

package daemon

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/internal/compare"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

var imagePath = "../tarball/testdata/test_image_1.tar"

type MockImageSaver struct {
	Client
	path       string
	negotiated bool
}

func (m *MockImageSaver) NegotiateAPIVersion(ctx context.Context) {
	m.negotiated = true
}

func (m *MockImageSaver) ImageSave(_ context.Context, _ []string) (io.ReadCloser, error) {
	if !m.negotiated {
		return nil, errors.New("you forgot to call NegotiateAPIVersion before calling ImageSave")

	}
	return os.Open(m.path)
}

func TestImage(t *testing.T) {
	for _, opts := range [][]ImageOption{{
		WithBufferedOpener(),
		WithClient(&MockImageSaver{path: imagePath}),
	}, {
		WithUnbufferedOpener(),
		WithClient(&MockImageSaver{path: imagePath}),
	}} {
		img, err := tarball.ImageFromPath(imagePath, nil)
		if err != nil {
			t.Fatalf("error loading test image: %s", err)
		}

		tag, err := name.NewTag("unused", name.WeakValidation)
		if err != nil {
			t.Fatalf("error creating test name: %s", err)
		}

		dmn, err := Image(tag, opts...)
		if err != nil {
			t.Fatalf("Error loading daemon image: %s", err)
		}
		if err := compare.Images(img, dmn); err != nil {
			t.Errorf("compare.Images: %v", err)
		}
	}
}
