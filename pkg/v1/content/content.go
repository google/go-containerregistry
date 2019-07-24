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

// Package content allows you to create images directly from contents.
package content

import (
	"archive/tar"
	"bytes"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Image creates a image with the given contents. These images are reproducible and should be consistent.
func Image(content ...map[string][]byte) (v1.Image, error) {

	tl := []v1.Layer{}
	for _, l := range content {
		b := &bytes.Buffer{}
		w := tar.NewWriter(b)
		for f, c := range l {
			if err := w.WriteHeader(&tar.Header{
				Name: f,
				Size: int64(len(c)),
			}); err != nil {
				return nil, err
			}
			if _, err := w.Write(c); err != nil {
				return nil, err
			}
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		y, err := tarball.LayerFromReader(b)
		if err != nil {
			return nil, err
		}
		tl = append(tl, y)
	}

	return mutate.AppendLayers(empty.Image, tl...)
}
