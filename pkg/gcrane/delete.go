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

package gcrane

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
)

// Delete deletes a remote image or index.
func Delete(ref string) error {
	// Same implementation of github.com/google/go-containerregistry/pkg/gcrane/copy
	original := authn.DefaultKeychain
	authn.DefaultKeychain = gcraneKeychain
	defer func() {
		authn.DefaultKeychain = original
	}()
	return crane.Delete(ref)
}
