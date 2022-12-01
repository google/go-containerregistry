// Copyright 2021 Google LLC All Rights Reserved.
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

package registry

import (
	"testing"

	"github.com/google/go-containerregistry/internal/depcheck"
)

func TestDeps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow depcheck")
	}
	depcheck.AssertOnlyDependencies(t, map[string][]string{
		"github.com/google/go-containerregistry/pkg/registry": append(
			depcheck.StdlibPackages(),
			"github.com/google/go-containerregistry/internal/httptest",
			"github.com/google/go-containerregistry/pkg/v1",
			"github.com/google/go-containerregistry/pkg/v1/types",

			"github.com/google/go-containerregistry/internal/verify",
			"github.com/google/go-containerregistry/internal/and",
		),
	})
}
