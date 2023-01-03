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
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
)

var (
	fresh              = 0
	testRegistry, _    = name.NewRegistry("test.io", name.WeakValidation)
	testRepo, _        = name.NewRepository("test.io/my-repo", name.WeakValidation)
	defaultRegistry, _ = name.NewRegistry(name.DefaultRegistry, name.WeakValidation)
)

func TestMain(m *testing.M) {
	// Set $HOME to a temp empty dir, to ensure $HOME/.docker/config.json
	// isn't unexpectedly found.
	tmp, err := os.MkdirTemp("", "keychain_test_home")
	if err != nil {
		log.Fatal(err)
	}
	os.Setenv("HOME", tmp)
	os.Exit(func() int {
		defer os.RemoveAll(tmp)
		return m.Run()
	}())
}

// setupConfigDir sets up an isolated configDir() for this test.
func setupConfigDir(t *testing.T) string {
	tmpdir := os.Getenv("TEST_TMPDIR")
	if tmpdir == "" {
		tmpdir = t.TempDir()
	}

	fresh++
	p := filepath.Join(tmpdir, fmt.Sprintf("%d", fresh))
	t.Logf("DOCKER_CONFIG=%s", p)
	t.Setenv("DOCKER_CONFIG", p)
	if err := os.Mkdir(p, 0777); err != nil {
		t.Fatalf("mkdir %q: %v", p, err)
	}
	return p
}

func setupConfigFile(t *testing.T, content string) string {
	cd := setupConfigDir(t)
	p := filepath.Join(cd, "config.json")
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatalf("write %q: %v", p, err)
	}

	// return the config dir so we can clean up
	return cd
}

func TestNoConfig(t *testing.T) {
	cd := setupConfigDir(t)
	defer os.RemoveAll(filepath.Dir(cd))

	auth, err := DefaultKeychain.Resolve(testRegistry)
	if err != nil {
		t.Fatalf("Resolve() = %v", err)
	}

	if auth != Anonymous {
		t.Errorf("expected Anonymous, got %v", auth)
	}
}

func TestPodmanConfig(t *testing.T) {
	tmpdir := os.Getenv("TEST_TMPDIR")
	if tmpdir == "" {
		tmpdir = t.TempDir()
	}
	fresh++
	p := filepath.Join(tmpdir, fmt.Sprintf("%d", fresh))
	t.Setenv("XDG_RUNTIME_DIR", p)
	os.Unsetenv("DOCKER_CONFIG")
	if err := os.MkdirAll(filepath.Join(p, "containers"), 0777); err != nil {
		t.Fatalf("mkdir %s/containers: %v", p, err)
	}
	cfg := filepath.Join(p, "containers/auth.json")
	content := fmt.Sprintf(`{"auths": {"test.io": {"auth": %q}}}`, encode("foo", "bar"))
	if err := os.WriteFile(cfg, []byte(content), 0600); err != nil {
		t.Fatalf("write %q: %v", cfg, err)
	}

	// At first, $DOCKER_CONFIG is unset and $HOME/.docker/config.json isn't
	// found, but Podman auth is configured. This should return Podman's
	// auth.
	auth, err := DefaultKeychain.Resolve(testRegistry)
	if err != nil {
		t.Fatalf("Resolve() = %v", err)
	}
	got, err := auth.Authorization()
	if err != nil {
		t.Fatal(err)
	}
	want := &AuthConfig{
		Username: "foo",
		Password: "bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	// Now, configure $HOME/.docker/config.json, which should override
	// Podman auth and be used.
	if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".docker"), 0777); err != nil {
		t.Fatalf("mkdir $HOME/.docker: %v", err)
	}
	cfg = filepath.Join(os.Getenv("HOME"), ".docker/config.json")
	content = fmt.Sprintf(`{"auths": {"test.io": {"auth": %q}}}`, encode("home-foo", "home-bar"))
	if err := os.WriteFile(cfg, []byte(content), 0600); err != nil {
		t.Fatalf("write %q: %v", cfg, err)
	}
	defer func() { os.Remove(cfg) }()
	auth, err = DefaultKeychain.Resolve(testRegistry)
	if err != nil {
		t.Fatalf("Resolve() = %v", err)
	}
	got, err = auth.Authorization()
	if err != nil {
		t.Fatal(err)
	}
	want = &AuthConfig{
		Username: "home-foo",
		Password: "home-bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	// Then, configure DOCKER_CONFIG with a valid config file with different
	// auth configured.
	// This demonstrates that DOCKER_CONFIG is preferred over Podman auth
	// and $HOME/.docker/config.json.
	content = fmt.Sprintf(`{"auths": {"test.io": {"auth": %q}}}`, encode("another-foo", "another-bar"))
	cd := setupConfigFile(t, content)
	defer os.RemoveAll(filepath.Dir(cd))

	auth, err = DefaultKeychain.Resolve(testRegistry)
	if err != nil {
		t.Fatalf("Resolve() = %v", err)
	}
	got, err = auth.Authorization()
	if err != nil {
		t.Fatal(err)
	}
	want = &AuthConfig{
		Username: "another-foo",
		Password: "another-bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func encode(user, pass string) string {
	delimited := fmt.Sprintf("%s:%s", user, pass)
	return base64.StdEncoding.EncodeToString([]byte(delimited))
}

func TestVariousPaths(t *testing.T) {
	tests := []struct {
		desc      string
		content   string
		wantErr   bool
		target    Resource
		cfg       *AuthConfig
		anonymous bool
	}{{
		desc:    "invalid config file",
		target:  testRegistry,
		content: `}{`,
		wantErr: true,
	}, {
		desc:    "creds store does not exist",
		target:  testRegistry,
		content: `{"credsStore":"#definitely-does-not-exist"}`,
		wantErr: true,
	}, {
		desc:    "valid config file",
		target:  testRegistry,
		content: fmt.Sprintf(`{"auths": {"test.io": {"auth": %q}}}`, encode("foo", "bar")),
		cfg: &AuthConfig{
			Username: "foo",
			Password: "bar",
		},
	}, {
		desc:    "valid config file; default registry",
		target:  defaultRegistry,
		content: fmt.Sprintf(`{"auths": {"%s": {"auth": %q}}}`, DefaultAuthKey, encode("foo", "bar")),
		cfg: &AuthConfig{
			Username: "foo",
			Password: "bar",
		},
	}, {
		desc:   "valid config file; matches registry w/ v1",
		target: testRegistry,
		content: fmt.Sprintf(`{
	  "auths": {
		"http://test.io/v1/": {"auth": %q}
	  }
	}`, encode("baz", "quux")),
		cfg: &AuthConfig{
			Username: "baz",
			Password: "quux",
		},
	}, {
		desc:   "valid config file; matches registry w/ v2",
		target: testRegistry,
		content: fmt.Sprintf(`{
	  "auths": {
		"http://test.io/v2/": {"auth": %q}
	  }
	}`, encode("baz", "quux")),
		cfg: &AuthConfig{
			Username: "baz",
			Password: "quux",
		},
	}, {
		desc:   "valid config file; matches repo",
		target: testRepo,
		content: fmt.Sprintf(`{
  "auths": {
    "test.io/my-repo": {"auth": %q},
    "test.io/another-repo": {"auth": %q},
    "test.io": {"auth": %q}
  }
}`, encode("foo", "bar"), encode("bar", "baz"), encode("baz", "quux")),
		cfg: &AuthConfig{
			Username: "foo",
			Password: "bar",
		},
	}, {
		desc:   "ignore unrelated repo",
		target: testRepo,
		content: fmt.Sprintf(`{
  "auths": {
    "test.io/another-repo": {"auth": %q},
	"test.io": {}
  }
}`, encode("bar", "baz")),
		cfg:       &AuthConfig{},
		anonymous: true,
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			cd := setupConfigFile(t, test.content)
			// For some reason, these tempdirs don't get cleaned up.
			defer os.RemoveAll(filepath.Dir(cd))

			auth, err := DefaultKeychain.Resolve(test.target)
			if test.wantErr {
				if err == nil {
					t.Fatal("wanted err, got nil")
				} else if err != nil {
					// success
					return
				}
			}
			if err != nil {
				t.Fatalf("wanted nil, got err: %v", err)
			}
			cfg, err := auth.Authorization()
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(cfg, test.cfg) {
				t.Errorf("got %+v, want %+v", cfg, test.cfg)
			}

			if test.anonymous != (auth == Anonymous) {
				t.Fatalf("unexpected anonymous authenticator")
			}
		})
	}
}

type helper struct {
	u, p string
	err  error
}

func (h helper) Get(serverURL string) (string, string, error) {
	if serverURL != "example.com" {
		return "", "", fmt.Errorf("unexpected serverURL: %s", serverURL)
	}
	return h.u, h.p, h.err
}

func TestNewKeychainFromHelper(t *testing.T) {
	var repo = name.MustParseReference("example.com/my/repo").Context()

	t.Run("success", func(t *testing.T) {
		kc := NewKeychainFromHelper(helper{"username", "password", nil})
		auth, err := kc.Resolve(repo)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", repo, err)
		}
		cfg, err := auth.Authorization()
		if err != nil {
			t.Fatalf("Authorization: %v", err)
		}
		if got, want := cfg.Username, "username"; got != want {
			t.Errorf("Username: got %q, want %q", got, want)
		}
		if got, want := cfg.IdentityToken, ""; got != want {
			t.Errorf("IdentityToken: got %q, want %q", got, want)
		}
		if got, want := cfg.Password, "password"; got != want {
			t.Errorf("Password: got %q, want %q", got, want)
		}
	})

	t.Run("success; identity token", func(t *testing.T) {
		kc := NewKeychainFromHelper(helper{"<token>", "idtoken", nil})
		auth, err := kc.Resolve(repo)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", repo, err)
		}
		cfg, err := auth.Authorization()
		if err != nil {
			t.Fatalf("Authorization: %v", err)
		}
		if got, want := cfg.Username, "<token>"; got != want {
			t.Errorf("Username: got %q, want %q", got, want)
		}
		if got, want := cfg.IdentityToken, "idtoken"; got != want {
			t.Errorf("IdentityToken: got %q, want %q", got, want)
		}
		if got, want := cfg.Password, ""; got != want {
			t.Errorf("Password: got %q, want %q", got, want)
		}
	})

	t.Run("failure", func(t *testing.T) {
		kc := NewKeychainFromHelper(helper{"", "", errors.New("oh no bad")})
		auth, err := kc.Resolve(repo)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", repo, err)
		}
		if auth != Anonymous {
			t.Errorf("Resolve: got %v, want %v", auth, Anonymous)
		}
	})
}

func TestConfigFileIsADir(t *testing.T) {
	tmpdir := setupConfigDir(t)
	// Create "config.json" as a directory, not a file to simulate optional
	// secrets in Kubernetes.
	err := os.Mkdir(path.Join(tmpdir, "config.json"), 0777)
	if err != nil {
		t.Fatal(err)
	}

	auth, err := DefaultKeychain.Resolve(testRegistry)
	if err != nil {
		t.Fatalf("Resolve() = %v", err)
	}
	if auth != Anonymous {
		t.Errorf("expected Anonymous, got %v", auth)
	}
}
