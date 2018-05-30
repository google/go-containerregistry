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
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/google/go-containerregistry/v1"
)

func TestReadOnlyCache(t *testing.T) {
	fs := failStore{}
	ro := ReadOnly(fs)
	h := mustDigest(t, randomImage(t))
	r := bytes.NewReader(nil)

	if _, err := fs.Load(h); err != nil {
		t.Error("Underlying Load: %v", err)
	}
	if err := fs.Store(h, r); err == nil {
		t.Error("Underlying Store, want err, got none")
	}

	if _, err := ro.Load(h); err != nil {
		t.Error("Readonly.Load: %v", err)
	}
	if err := ro.Store(h, r); err != nil {
		t.Errorf("ReadOnly.Store: %v", err)
	}
}

type failStore struct{}

func (failStore) Load(v1.Hash) (io.ReadCloser, error) { return nil, nil }
func (failStore) Store(v1.Hash, io.Reader) error      { return errors.New("cannot store") }
