// Copyright 2019 Google LLC All Rights Reserved.
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
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Tag adds one or more tags to the remote img.
func Tag(img string, tags any, opt ...Option) error {
	o := makeOptions(opt...)
	ref, err := name.ParseReference(img, o.Name...)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %w", img, err)
	}
	desc, err := remote.Get(ref, o.Remote...)
	if err != nil {
		return fmt.Errorf("fetching %q: %w", img, err)
	}

	// Handle both single tag (string) and multiple tags ([]string) for backwards compatibility
	var tagList []string
	switch t := tags.(type) {
	case string:
		tagList = []string{t}
	case []string:
		tagList = t
	default:
		return fmt.Errorf("tags must be string or []string, got %T", tags)
	}

	// Apply each tag
	for i, tag := range tagList {
		dst := ref.Context().Tag(tag)
		if err := remote.Tag(dst, desc, o.Remote...); err != nil {
			if i > 0 {
				return fmt.Errorf("tagging %q with %q failed (successfully tagged with %v): %w", img, tag, tagList[:i], err)
			}
			return fmt.Errorf("tagging %q with %q: %w", img, tag, err)
		}
	}

	return nil
}
