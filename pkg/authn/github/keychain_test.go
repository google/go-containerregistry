// Copyright 2022 Google LLC All Rights Reserved.
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

package github

import (
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
)

// TestKeychain checks that the keychain resolves when $GITHUB_TOKEN is set and
// the request is for GHCR.
func TestKeychain(t *testing.T) {
	username, tok := "octocat", "my-token"
	os.Setenv("GITHUB_ACTOR", username)
	os.Setenv("GITHUB_TOKEN", tok)
	got, err := Keychain.Resolve(resource("ghcr.io/my/repo"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got == authn.Anonymous {
		t.Fatalf("Got anonymous, wanted authenticator")
	}

	auth, err := got.Authorization()
	if err != nil {
		t.Fatalf("Authorization: %v", err)
	}
	if auth.Username != username {
		t.Errorf("Got username %q, want %q", auth.Username, username)
	}
	if auth.Password != tok {
		t.Errorf("Got password %q, want %q", auth.Password, tok)
	}
}

// TestKeychainUsernameUnset checks that the keychain resolves an "unset"
// username when $GITHUB_ACTOR is not set.
func TestKeychainUsernameUnset(t *testing.T) {
	tok := "my-token"
	os.Unsetenv("GITHUB_ACTOR")
	os.Setenv("GITHUB_TOKEN", tok)
	got, err := Keychain.Resolve(resource("ghcr.io/my/repo"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got == authn.Anonymous {
		t.Fatalf("Got anonymous, wanted authenticator")
	}

	auth, err := got.Authorization()
	if err != nil {
		t.Fatalf("Authorization: %v", err)
	}
	if auth.Username != "unset" {
		t.Errorf("Got username %q, want unset", auth.Username)
	}
	if auth.Password != tok {
		t.Errorf("Got password %q, want %q", auth.Password, tok)
	}
}

// TestKeychainUnset checks that the keychain doesn't resolve when the
// environment variable is unset.
func TestKeychainUnset(t *testing.T) {
	os.Unsetenv("GITHUB_TOKEN")

	got, err := Keychain.Resolve(resource("ghcr.io/my/repo"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != authn.Anonymous {
		t.Errorf("Resolve(ghcr.io) got %v, want Anonymous", got)
	}
}

// TestNoMatch checks that the keychain doesn't resolve for non-GHCR registries.
func TestNoMatch(t *testing.T) {
	os.Setenv("GITHUB_TOKEN", "my-token")
	for _, s := range []string{
		"gcr.io",
		"example.com",
		"ghcr.io.example.com",
		"invalid-domain-name -- %U)(@*)(%*)@(*#%@",
	} {
		got, err := Keychain.Resolve(resource(s))
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if got != authn.Anonymous {
			t.Errorf("Resolve(%q) got %v, want Anonymous", s, got)
		}
	}
}

type resource string

func (r resource) String() string      { return string(r) }
func (r resource) RegistryStr() string { return string(r) }
