// Copyright 2022 Google LLC All Rights Reserved.
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
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Referrers returns descriptors that refer to a given image reference.
//
// New attachments can be added with crane.Attach.
func Referrers(refstr string, opt ...Option) ([]v1.Descriptor, error) {
	o := makeOptions(opt...)

	var dig name.Digest
	ref, err := name.ParseReference(refstr, o.Name...)
	if err != nil {
		return nil, err
	}
	if digr, ok := ref.(name.Digest); ok {
		dig = digr
	} else {
		desc, err := remote.Head(ref, o.Remote...)
		if err != nil {
			// If you asked for a tag and it doesn't exist, we can't help you.
			return nil, err
		}
		dig = ref.Context().Digest(desc.Digest.String())
	}

	return remote.Referrers(dig, o.Remote...)
}
