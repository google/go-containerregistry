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

package v1

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseConfig(t *testing.T) {
	got, err := ParseConfigFile(strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	want := ConfigFile{}

	if diff := cmp.Diff(want, *got); diff != "" {
		t.Errorf("ParseConfigFile({}); (-want +got) %s", diff)
	}

	if got, err := ParseConfigFile(strings.NewReader("{")); err == nil {
		t.Errorf("expected error, got: %v", got)
	}
}
