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

package remote

import (
	"errors"
	"io"

	"github.com/google/go-containerregistry/pkg/v1"
)

type StreamableLayer struct {
	Blob io.ReadCloser
}

var _ v1.Layer = (*StreamableLayer)(nil)

func (s *StreamableLayer) Digest() (v1.Hash, error) {
	return v1.Hash{}, errors.New("don't know digest of streamed layer")
}

func (s *StreamableLayer) DiffID() (v1.Hash, error) {
	return v1.Hash{}, errors.New("don't know digest of streamed layer")
}

func (s *StreamableLayer) Uncompressed() (io.ReadCloser, error) {
	return nil, errors.New("refusing to decompress streamed layer")
}

func (s *StreamableLayer) Compressed() (io.ReadCloser, error) { return s.Blob, nil }

func (s *StreamableLayer) Size() (int64, error) {
	return 0, errors.New("don't know size of streamed layer")
}
