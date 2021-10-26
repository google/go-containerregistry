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

package registry_test

import (
	"fmt"
	"net/http/httptest"

	"github.com/google/go-containerregistry/pkg/registry"
)

func Example() {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	resp, _ := s.Client().Get(s.URL + "/v2/bar/blobs/sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae")
	fmt.Println(resp.StatusCode)
	// Output: 404
}
