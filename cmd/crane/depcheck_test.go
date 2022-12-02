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

package main

import (
	"testing"

	"github.com/google/go-containerregistry/internal/depcheck"
)

func TestDeps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow depcheck")
	}
	depcheck.AssertNoDependency(t, map[string][]string{
		"github.com/google/go-containerregistry/cmd/crane": {
			"github.com/google/go-containerregistry/pkg/v1/daemon",
		},
	})
}
