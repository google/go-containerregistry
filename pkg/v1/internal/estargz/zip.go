// Copyright 2020 Google LLC All Rights Reserved.
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

package estargz

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/containerd/stargz-snapshotter/estargz"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// ReadCloser reads uncompressed tarball input from the io.ReadCloser and
// returns:
//  * An io.ReadCloser from which compressed data may be read, and
//  * A v1.Hash with the hash of the estargz table of contents, or
//  * An error if the estargz processing encountered a problem.
//
// Refer to estargz for the options:
// https://pkg.go.dev/github.com/containerd/stargz-snapshotter@v0.2.0/estargz#Option
func ReadCloser(r io.ReadCloser, opts ...estargz.Option) (io.ReadCloser, v1.Hash, error) {
	defer r.Close()

	bs, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, v1.Hash{}, err
	}
	br := bytes.NewReader(bs)

	rc, d, err := estargz.Build(io.NewSectionReader(br, 0, int64(len(bs))), nil, opts...)
	if err != nil {
		return nil, v1.Hash{}, err
	}
	h, err := v1.NewHash(d.String())
	return rc, h, err
}
