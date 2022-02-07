//go:build !arm64
// +build !arm64

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

package google

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"golang.org/x/oauth2"
)

const (
	// Fails to parse as JSON at all.
	badoutput = ""

	// Fails to parse token_expiry format.
	badexpiry = `
{
  "credential": {
    "access_token": "mytoken",
    "token_expiry": "most-definitely-not-a-date"
  }
}`

	// Expires in 6,000 years. Hopefully nobody is using software then.
	success = `
{
  "credential": {
    "access_token": "mytoken",
    "token_expiry": "8018-12-02T04:08:13Z"
  }
}`
)

// We'll invoke ourselves with a special environment variable in order to mock
// out the gcloud dependency of gcloudSource. The exec package does this, too.
//
// See: https://www.joeshaw.org/testing-with-os-exec-and-testmain/
//
// TODO(#908): This doesn't work on arm64 or darwin for some reason.
func TestMain(m *testing.M) {
	switch os.Getenv("GO_TEST_MODE") {
	case "":
		// Normal test mode
		os.Exit(m.Run())

	case "error":
		// Makes cmd.Run() return an error.
		os.Exit(2)

	case "badoutput":
		// Makes the gcloudOutput Unmarshaler fail.
		fmt.Println(badoutput)

	case "badexpiry":
		// Makes the token_expiry time parser fail.
		fmt.Println(badexpiry)

	case "success":
		// Returns a seemingly valid token.
		fmt.Println(success)
	}
}

func newGcloudCmdMock(env string) func() *exec.Cmd {
	return func() *exec.Cmd {
		cmd := exec.Command(os.Args[0])
		cmd.Env = []string{fmt.Sprintf("GO_TEST_MODE=%s", env)}
		return cmd
	}
}

func TestGcloudErrors(t *testing.T) {
	cases := []struct {
		env string

		// Just look for the prefix because we can't control other packages' errors.
		wantPrefix string
	}{{
		env:        "error",
		wantPrefix: "error executing `gcloud config config-helper`:",
	}, {
		env:        "badoutput",
		wantPrefix: "failed to parse `gcloud config config-helper` output:",
	}, {
		env:        "badexpiry",
		wantPrefix: "failed to parse gcloud token expiry:",
	}}

	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			GetGcloudCmd = newGcloudCmdMock(tc.env)

			if _, err := NewGcloudAuthenticator(); err == nil {
				t.Errorf("wanted error, got nil")
			} else if got := err.Error(); !strings.HasPrefix(got, tc.wantPrefix) {
				t.Errorf("wanted error prefix %q, got %q", tc.wantPrefix, got)
			}
		})
	}
}

func TestGcloudSuccess(t *testing.T) {
	// Stupid coverage to make sure it doesn't panic.
	var b bytes.Buffer
	logs.Debug.SetOutput(&b)

	GetGcloudCmd = newGcloudCmdMock("success")

	auth, err := NewGcloudAuthenticator()
	if err != nil {
		t.Fatalf("NewGcloudAuthenticator got error %v", err)
	}

	token, err := auth.Authorization()
	if err != nil {
		t.Fatalf("Authorization got error %v", err)
	}

	if got, want := token.Password, "mytoken"; got != want {
		t.Errorf("wanted token %q, got %q", want, got)
	}
}

//
// Keychain tests are in here so we can reuse the fake gcloud stuff.
//

func mustRegistry(r string) name.Registry {
	reg, err := name.NewRegistry(r, name.StrictValidation)
	if err != nil {
		panic(err)
	}
	return reg
}

func TestKeychainDockerHub(t *testing.T) {
	if auth, err := Keychain.Resolve(mustRegistry("index.docker.io")); err != nil {
		t.Errorf("expected success, got: %v", err)
	} else if auth != authn.Anonymous {
		t.Errorf("expected anonymous, got: %v", auth)
	}
}

func TestKeychainGCRandAR(t *testing.T) {
	cases := []struct {
		host       string
		expectAuth bool
	}{
		// GCR hosts
		{"gcr.io", true},
		{"us.gcr.io", true},
		{"eu.gcr.io", true},
		{"asia.gcr.io", true},
		{"staging-k8s.gcr.io", true},
		{"global.gcr.io", true},
		{"notgcr.io", false},
		{"fake-gcr.io", false},
		{"alsonot.gcr.iot", false},
		// AR hosts
		{"us-docker.pkg.dev", true},
		{"asia-docker.pkg.dev", true},
		{"europe-docker.pkg.dev", true},
		{"us-central1-docker.pkg.dev", true},
		{"us-docker-pkg.dev", false},
		{"someotherpkg.dev", false},
		{"looks-like-pkg.dev", false},
		{"closeto.pkg.devops", false},
	}

	// Env should fail.
	if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/dev/null"); err != nil {
		t.Fatalf("unexpected err os.Setenv: %v", err)
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("cases[%d]", i), func(t *testing.T) {
			// Reset the keychain to ensure we don't cache earlier results.
			Keychain = &googleKeychain{}

			// Gcloud should succeed.
			GetGcloudCmd = newGcloudCmdMock("success")

			if auth, err := Keychain.Resolve(mustRegistry(tc.host)); err != nil {
				t.Errorf("expected success for %v, got: %v", tc.host, err)
			} else if tc.expectAuth && auth == authn.Anonymous {
				t.Errorf("expected not anonymous auth for %v, got: %v", tc, auth)
			} else if !tc.expectAuth && auth != authn.Anonymous {
				t.Errorf("expected anonymous auth for %v, got: %v", tc, auth)
			}

			// Make gcloud fail to test that caching works.
			GetGcloudCmd = newGcloudCmdMock("badoutput")

			if auth, err := Keychain.Resolve(mustRegistry(tc.host)); err != nil {
				t.Errorf("expected success for %v, got: %v", tc.host, err)
			} else if tc.expectAuth && auth == authn.Anonymous {
				t.Errorf("expected not anonymous auth for %v, got: %v", tc, auth)
			} else if !tc.expectAuth && auth != authn.Anonymous {
				t.Errorf("expected anonymous auth for %v, got: %v", tc, auth)
			}
		})
	}
}

func TestKeychainError(t *testing.T) {
	if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/dev/null"); err != nil {
		t.Fatalf("unexpected err os.Setenv: %v", err)
	}

	GetGcloudCmd = newGcloudCmdMock("badoutput")

	// Reset the keychain to ensure we don't cache earlier results.
	Keychain = &googleKeychain{}
	if auth, err := Keychain.Resolve(mustRegistry("gcr.io")); err != nil {
		t.Fatalf("got error: %v", err)
	} else if auth != authn.Anonymous {
		t.Fatalf("wanted Anonymous, got %v", auth)
	}
}

type badSource struct{}

func (bs badSource) Token() (*oauth2.Token, error) {
	return nil, fmt.Errorf("oops")
}

// This test is silly, but coverage.
func TestTokenSourceAuthError(t *testing.T) {
	auth := tokenSourceAuth{badSource{}}

	_, err := auth.Authorization()
	if err == nil {
		t.Errorf("expected err, got nil")
	}
}

func TestNewEnvAuthenticatorFailure(t *testing.T) {
	if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/dev/null"); err != nil {
		t.Fatalf("unexpected err os.Setenv: %v", err)
	}

	// Expect error.
	_, err := NewEnvAuthenticator()
	if err == nil {
		t.Errorf("expected err, got nil")
	}
}
