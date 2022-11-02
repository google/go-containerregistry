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
	"errors"
	"net/http"

	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Attach attaches contents to an image reference with the given media type.
//
// These attached descriptors can be listed using crane.Referrers.
func Attach(refstr string, b []byte, mediaType string, opt ...Option) error {
	o := makeOptions(opt...)

	var dig name.Digest
	ref, err := name.ParseReference(refstr, o.Name...)
	if err != nil {
		return err
	}
	if digr, ok := ref.(name.Digest); ok {
		dig = digr
	} else {
		desc, err := remote.Head(ref, o.Remote...)
		if err != nil {
			return err
		}
		dig = ref.Context().Digest(desc.Digest.String())
	}

	desc, err := remote.Head(dig, o.Remote...)
	var terr *transport.Error
	if errors.As(err, &terr) && terr.StatusCode == http.StatusNotFound {
		h, err := v1.NewHash(dig.DigestStr())
		if err != nil {
			return err
		}
		// The subject doesn't exist, attach to it as if it's an empty OCI image.
		logs.Progress.Println("subject doesn't exist, attaching to empty image")
		desc = &v1.Descriptor{
			MediaType: types.OCIManifestSchema1,
			Size:      0,
			Digest:    h,
		}
	} else if err != nil {
		return err
	}

	att, err := mutate.AppendLayers(
		mutate.MediaType(empty.Image, types.OCIManifestSchema1),
		static.NewLayer(b, types.MediaType(mediaType)))
	if err != nil {
		return err
	}
	att = mutate.Subject(att, *desc).(v1.Image)
	attdig, err := att.Digest()
	if err != nil {
		return err
	}
	attref := ref.Context().Digest(attdig.String())
	return remote.Write(attref, att, o.Remote...)
}
