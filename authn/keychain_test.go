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
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/google/go-containerregistry/name"
)

func TestConfigDir(t *testing.T) {
	clearEnv := func() {
		for _, e := range []string{"HOME", "DOCKER_CONFIG", "HOMEDRIVE", "HOMEPATH"} {
			os.Unsetenv(e)
		}
	}

	t.Run("no envs", func(t *testing.T) {
		// Without any envs set, should return an error.
		clearEnv()
		if got, err := configDir(); err == nil {
			t.Errorf("configDir() returned no error, got %q", got)
		}
	})

	t.Run("DOCKER_CONFIG", func(t *testing.T) {
		// DOCKER_CONFIG is set and is returned.
		clearEnv()
		want := "/path/to/.docker"
		os.Setenv("DOCKER_CONFIG", want)
		if got, err := configDir(); err != nil {
			t.Errorf("configDir(): %v", err)
		} else if got != want {
			t.Errorf("configDir() got %v, want %v", got, want)
		}
	})

	t.Run("HOME", func(t *testing.T) {
		// DOCKER_CONFIG is unset, but HOME is.
		if runtime.GOOS == "windows" {
			t.Skip("Not running on Windows")
		}
		clearEnv()
		os.Setenv("HOME", "/my/home")
		want := "/my/home/.docker"
		if got, err := configDir(); err != nil {
			t.Errorf("configDir(): %v", err)
		} else if got != want {
			t.Errorf("configDir() got %q, want %q", got, want)
		}
	})

	t.Run("USERPROFILE", func(t *testing.T) {
		// DOCKER_CONFIG and HOME are unset, but USERPROFILE is.
		clearEnv()
		os.Setenv("USERPROFILE", "/user/profile")
		want := "/user/profile/.docker"
		if got, err := configDir(); err != nil {
			t.Errorf("configDir(): %v", err)
		} else if got != want {
			t.Errorf("configDir() got %q, want %q", got, want)
		}
	})
}

var (
	fresh           = 0
	testRegistry, _ = name.NewRegistry("test.io", name.WeakValidation)
)

// setupConfigDir sets up an isolated configDir() for this test.
func setupConfigDir() string {
	fresh = fresh + 1
	p := fmt.Sprintf("%s/%d", os.Getenv("TEST_TMPDIR"), fresh)
	os.Setenv("DOCKER_CONFIG", p)
	if err := os.Mkdir(p, 0777); err != nil {
		panic(err)
	}
	return p
}

func setupConfigFile(content string) {
	p := path.Join(setupConfigDir(), "config.json")
	if err := ioutil.WriteFile(p, []byte(content), 0600); err != nil {
		panic(err)
	}
}

func checkOutput(t *testing.T, want string) {
	auth, err := DefaultKeychain.Resolve(testRegistry)
	if err != nil {
		t.Errorf("Resolve() = %v", err)
	}

	got, err := auth.Authorization()
	if err != nil {
		t.Errorf("Authorization() = %v", err)
	}
	if got != want {
		t.Errorf("Authorization(); got %v, want %v", got, want)
	}
}

func checkAnonymousFallback(t *testing.T) {
	checkOutput(t, "")
}

func checkFooBarOutput(t *testing.T) {
	// base64(foo:bar)
	checkOutput(t, "Basic Zm9vOmJhcg==")
}

func checkHelper(t *testing.T) {
	auth, err := DefaultKeychain.Resolve(testRegistry)
	if err != nil {
		t.Errorf("Resolve() = %v", err)
	}

	help, ok := auth.(*helper)
	if !ok {
		t.Errorf("Resolve(); got %T, want *helper", auth)
	}
	if help.name != "test" {
		t.Errorf("Resolve().name; got %v, want \"test\"", help.name)
	}
	if help.domain != testRegistry {
		t.Errorf("Resolve().domain; got %v, want %v", help.domain, testRegistry)
	}
}

func TestNoConfig(t *testing.T) {
	setupConfigDir()

	checkAnonymousFallback(t)
}

func TestVariousPaths(t *testing.T) {
	tests := []struct {
		content string
		check   func(*testing.T)
	}{{
		content: `}{`,
		check:   checkAnonymousFallback,
	}, {
		content: `{"credHelpers": {"https://test.io": "test"}}`,
		check:   checkHelper,
	}, {
		content: `{"credsStore": "test"}`,
		check:   checkHelper,
	}, {
		content: `{"auths": {"http://test.io/v2/": {"auth": "Zm9vOmJhcg=="}}}`,
		check:   checkFooBarOutput,
	}, {
		content: `{"auths": {"https://test.io/v1/": {"username": "foo", "password": "bar"}}}`,
		check:   checkFooBarOutput,
	}, {
		content: `{"auths": {"other.io": {"username": "asdf", "password": "fdsa"}}}`,
		check:   checkAnonymousFallback,
	}}

	for _, test := range tests {
		setupConfigFile(test.content)

		test.check(t)
	}
}
