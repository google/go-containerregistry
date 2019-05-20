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

package empty

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestIndex(t *testing.T) {
	if err := validate.Index(Index); err != nil {
		t.Fatalf("validate.Index(empty.Index) = %v", err)
	}

	if mt, err := Index.MediaType(); err != nil || mt != types.OCIImageIndex {
		t.Errorf("empty.Index.MediaType() = %v, %v", mt, err)
	}

	if _, err := Index.Image(v1.Hash{}); err == nil {
		t.Errorf("empty.Index.Image() should always fail")
	}
	if _, err := Index.ImageIndex(v1.Hash{}); err == nil {
		t.Errorf("empty.Index.ImageIndex() should always fail")
	}
}
