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

// +build integration

package remote

import (
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

func TestCheckPushPermission_Real(t *testing.T) {
	// Tests should not run in an environment where these registries can
	// be pushed to.
	for _, r := range []name.Reference{
		mustNewTag(t, "ubuntu"),
		mustNewTag(t, "google/cloud-sdk"),
		mustNewTag(t, "microsoft/dotnet:sdk"),
		mustNewTag(t, "gcr.io/non-existent-project/made-up"),
		mustNewTag(t, "gcr.io/google-containers/foo"),
		mustNewTag(t, "quay.io/username/reponame"),
	} {
		t.Run(r.String(), func(t *testing.T) {
			t.Parallel()
			if err := CheckPushPermission(r, authn.DefaultKeychain, http.DefaultTransport); err == nil {
				t.Errorf("CheckPushPermission(%s) returned nil", r)
			}
		})
	}
}
