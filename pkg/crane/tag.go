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
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"golang.org/x/mod/semver"
)

// Tag adds tag to the remote img.
func Tag(img, tag string, opt ...Option) error {
	o := makeOptions(opt...)
	ref, err := name.ParseReference(img, o.Name...)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %w", img, err)
	}
	desc, err := remote.Get(ref, o.Remote...)
	if err != nil {
		return fmt.Errorf("fetching %q: %w", img, err)
	}

	dst := ref.Context().Tag(tag)

	return remote.Tag(dst, desc, o.Remote...)
}

// Bump adds tag to the remote img and updates any tags that need to be
// updated per semver.
func Bump(img, tag string, opt ...Option) ([]string, error) {
	o := makeOptions(opt...)

	ref, err := name.ParseReference(img, o.Name...)
	if err != nil {
		return nil, fmt.Errorf("parsing reference %q: %w", img, err)
	}

	if !semver.IsValid(tag) {
		return nil, fmt.Errorf("%q is not valid semver", tag)
	}
	if semver.Build(tag) != "" {
		return nil, fmt.Errorf("%q is not a valid registry tag", tag)
	}

	desc, err := remote.Get(ref, o.Remote...)
	if err != nil {
		return nil, fmt.Errorf("fetching %q: %w", img, err)
	}

	if semver.Prerelease(tag) != "" {
		// TODO: Prerelease shouldn't bump stuff. Log warning?
		return nil, remote.Tag(ref.Context().Tag(tag), desc, o.Remote...)
	}

	tags, err := remote.List(ref.Context(), o.Remote...)
	if err != nil {
		return nil, err
	}

	// Gather all the actually valid semver tags and sort them.
	released := []string{}
	for _, t := range tags {
		if semver.IsValid(t) && semver.Prerelease(t) == "" {
			released = append(released, t)
		}
	}
	semver.Sort(released)

	todo := []string{tag}
	// Bump "vMAJOR.MINOR" if needed.
	if ok, mm := shouldBump(released, tag, semver.MajorMinor); ok {
		todo = append(todo, mm)
	}
	// Bump "vMAJOR" if needed.
	if ok, m := shouldBump(released, tag, semver.Major); ok {
		todo = append(todo, m)
	}
	// Bump "latest" if needed.
	if len(released) > 0 {
		max := released[len(released)-1]
		if semver.Compare(tag, max) == 1 {
			todo = append(todo, "latest")
		}
	}

	pushed := []string{}
	for _, t := range todo {
		src := ref.Context().Tag(t)
		rmt, headErr := remote.Head(src, o.Remote...)
		if headErr != nil {
			var terr *transport.Error
			if !errors.As(headErr, &terr) || terr.StatusCode != http.StatusNotFound {
				return pushed, headErr
			}
		}

		// If we got a 404 or the existing tag isn't right.
		if headErr != nil || rmt.Digest != desc.Digest {
			if err := remote.Tag(src, desc, o.Remote...); err != nil {
				return pushed, err
			}

			if headErr == nil {
				oref := src.String() + "@" + rmt.Digest.String()
				pushed = append(pushed, oref)
			} else {
				pushed = append(pushed, src.String())
			}
		}
	}

	return pushed, nil
}

func shouldBump(released []string, tag string, versionFunc func(string) string) (bool, string) {
	target := versionFunc(tag)
	for _, t := range released {
		if versionFunc(t) == target && semver.Compare(tag, t) == -1 {
			// An existing tag should have already bumped this version.
			return false, ""
		}
	}
	return true, target
}
