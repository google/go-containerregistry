// Copyright 2026 Google LLC All Rights Reserved.
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

package remote

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

type dockerConfig struct {
	Auths map[string]dockerAuth `json:"auths"`
}

type dockerAuth struct {
	Auth string `json:"auth"`
}

func basicAuth(username, password string) dockerAuth {
	return dockerAuth{Auth: base64.StdEncoding.EncodeToString([]byte(username + ":" + password))}
}

// TestCheckAgreesWithWrite asserts that CheckPushPermission and Write always agree.
func TestCheckAgreesWithWrite(t *testing.T) {
	const (
		repo = "write/time"
		user = "user"
		pass = "pass"
	)

	reg := registry.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, ok := r.BasicAuth()
		if r.URL.Path != "/v2/" && (!ok || gotUser != user || gotPass != pass) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		reg.ServeHTTP(w, r)
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}
	host := u.Host

	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatalf("random.Image() = %v", err)
	}

	ref := mustNewTag(t, fmt.Sprintf("%s/%s:latest", host, repo))

	cases := []struct {
		name  string
		auths map[string]dockerAuth
	}{{
		"registry credential",
		map[string]dockerAuth{
			host: basicAuth(user, pass),
		},
	}, {
		"repository credential",
		map[string]dockerAuth{
			host + "/" + repo: basicAuth(user, pass),
		},
	}, {
		"repository credential alongside a registry credential",
		map[string]dockerAuth{
			host:              basicAuth("wrong", "wrong"),
			host + "/" + repo: basicAuth(user, pass),
		},
	}, {
		"unusable repository credential alongside a registry credential",
		map[string]dockerAuth{
			host:              basicAuth(user, pass),
			host + "/" + repo: basicAuth("wrong", "wrong"),
		},
	}, {
		"no usable credential",
		map[string]dockerAuth{
			host: basicAuth("wrong", "wrong"),
		},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			config, err := json.Marshal(dockerConfig{Auths: c.auths})
			if err != nil {
				t.Fatalf("json.Marshal() = %v", err)
			}

			dir := t.TempDir()
			t.Setenv("DOCKER_CONFIG", dir)
			err = os.WriteFile(filepath.Join(dir, "config.json"), config, 0600)
			if err != nil {
				t.Fatalf("writing config.json: %v", err)
			}

			checkErr := CheckPushPermission(ref, authn.DefaultKeychain, http.DefaultTransport)
			writeErr := Write(ref, img, WithAuthFromKeychain(authn.DefaultKeychain))
			if (checkErr != nil) != (writeErr != nil) {
				t.Errorf("CheckPushPermission and Write disagree: check = %v, write = %v", checkErr, writeErr)
			}
		})
	}
}
