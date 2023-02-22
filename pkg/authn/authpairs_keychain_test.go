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

package authn

import (
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
)

func TestAuthPairsKeychainResolvesWithAuthPairs(t *testing.T) {
	var (
		repo1, _ = name.NewRepository("test.io/my-repo1", name.WeakValidation)
		repo2, _ = name.NewRepository("test.io/my-repo2", name.WeakValidation)
		repo3, _ = name.NewRepository("test.io/my-repo3", name.WeakValidation)

		// Set up the default docker config dir last, so DOCKER_CONFIG env var is set to an empty dir
		cd1 = setupConfigFile(t, fmt.Sprintf(`{"auths": {"test.io": {"auth": %q}}}`, encode("user1", "pass1")))
		cd2 = setupConfigFile(t, fmt.Sprintf(`{"auths": {"test.io": {"auth": %q}}}`, encode("user2", "pass2")))
		_   = setupConfigDir(t)
	)

	kc := NewAuthPairsKeychain(WithAuthPairs(map[string]string{
		repo1.String(): cd1,
		repo2.String(): cd2,
	}))

	checkResolveRepo := func(t *testing.T, repo name.Repository, want string) {
		t.Helper()

		auth, err := kc.Resolve(repo)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", repo, err)
		}

		if want == "" {
			if auth != Anonymous {
				t.Errorf("expected Anonymous, got %v", auth)
			}
			return
		}

		cfg, err := auth.Authorization()
		if err != nil {
			t.Fatalf("Authorization: %v", err)
		}
		if got := fmt.Sprintf("%s:%s", cfg.Username, cfg.Password); got != want {
			t.Errorf("Auth: got %q, want %q", got, want)
		}
	}

	checkResolveRepo(t, repo1, "user1:pass1")
	checkResolveRepo(t, repo2, "user2:pass2")
	checkResolveRepo(t, repo3, "")
}
