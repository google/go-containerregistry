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

	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Copy copies a remote image or index from src to dst.
func Copy(src, dst string, opt ...Option) error {
	o := makeOptions(opt...)
	srcRef, err := name.ParseReference(src, o.Name...)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %w", src, err)
	}

	dstRef, err := name.ParseReference(dst, o.Name...)
	if err != nil {
		return fmt.Errorf("parsing reference for %q: %w", dst, err)
	}

	pusher, err := remote.NewPusher(o.Remote...)
	if err != nil {
		return err
	}

	puller, err := remote.NewPuller(o.Remote...)
	if err != nil {
		return err
	}

	logs.Progress.Printf("Copying from %v to %v", srcRef, dstRef)
	desc, err := puller.Get(o.ctx, srcRef)
	if err != nil {
		return fmt.Errorf("fetching %q: %w", src, err)
	}

	if o.Platform == nil {
		return pusher.Push(o.ctx, dstRef, desc)
	}

	// If platform is explicitly set, don't copy the whole index, just the appropriate image.
	img, err := desc.Image()
	if err != nil {
		return err
	}
	return pusher.Push(o.ctx, dstRef, img)
}
