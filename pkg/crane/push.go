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

package crane

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Load reads the tarball at path as a v1.Image.
func Load(path string) (v1.Image, error) {
	// TODO: Allow tag?
	return tarball.ImageFromPath(path, nil)
}

// Push pushes the v1.Image img to a registry as dst.
func Push(img v1.Image, dst string, opt ...Option) error {
	o := makeOptions(opt...)
	tag, err := name.NewTag(dst, o.name...)
	if err != nil {
		return fmt.Errorf("parsing tag %q: %v", dst, err)
	}
	return remote.Write(tag, img, o.remote...)
}

// PushIndex pushes the v1.ImageIndex index to a registry as dst.
func PushIndex(idx v1.ImageIndex, dst string, opt ...Option) error {
	o := makeOptions(opt...)
	dstRef, err := name.ParseReference(dst, o.name...)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", dst, err)
	}
	return remote.WriteIndex(dstRef, idx, o.remote...)
}
