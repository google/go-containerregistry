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

package build

import (
	"io/ioutil"
	"os"
	"path"
	"time"

	"testing"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

type testContext struct {
	gopath  string
	workdir string
}

func (tc *testContext) Enter(t *testing.T) {
	// Track the original state, so that we can restore it.
	ogp := os.Getenv("GOPATH")
	// Change the current state for the test.
	os.Setenv("GOPATH", tc.gopath)
	getwd = func() (string, error) {
		return tc.workdir, nil
	}
	// Record the original state for restoration.
	tc.gopath = ogp
}

func (tc *testContext) Exit(t *testing.T) {
	// Restore the original state.
	os.Setenv("GOPATH", tc.gopath)
	getwd = os.Getwd
}

func TestComputeImportPath(t *testing.T) {
	tests := []struct {
		desc             string
		ctx              testContext
		expectErr        bool
		expectImportpath string
	}{{
		desc: "simple gopath",
		ctx: testContext{
			gopath:  "/go",
			workdir: "/go/src/github.com/foo/bar",
		},
		expectImportpath: "github.com/foo/bar",
	}, {
		desc: "trailing slashes",
		ctx: testContext{
			gopath:  "/go/",
			workdir: "/go/src/github.com/foo/bar/",
		},
		expectImportpath: "github.com/foo/bar",
	}, {
		desc: "not on gopath",
		ctx: testContext{
			gopath:  "/go",
			workdir: "/rust/src/github.com/foo/bar",
		},
		expectErr: true,
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Set the context for our test.
			test.ctx.Enter(t)
			defer test.ctx.Exit(t)

			ip, err := computeImportpath()
			if err != nil && !test.expectErr {
				t.Errorf("computeImportpath() = %v, want %v", err, test.expectImportpath)
			} else if err == nil && test.expectErr {
				t.Errorf("computeImportpath() = %v, want error", ip)
			} else if err == nil && !test.expectErr {
				if got, want := ip, test.expectImportpath; want != got {
					t.Errorf("computeImportpath() = %v, want %v", got, want)
				}
			}
		})
	}
}

func TestGoBuildIsSupportedRef(t *testing.T) {
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image() = %v", err)
	}
	importpath := "github.com/google/go-containerregistry"
	tc := testContext{
		gopath:  "/go",
		workdir: "/go/src/" + importpath,
	}
	tc.Enter(t)
	defer tc.Exit(t)
	ng, err := NewGo(Options{GetBase: func(string) (v1.Image, error) {
		return img, nil
	}})
	if err != nil {
		t.Fatalf("NewGo() = %v", err)
	}

	supportedTests := []string{
		path.Join(importpath, "pkg", "foo"),
		path.Join(importpath, "cmd", "crane"),
	}

	for _, test := range supportedTests {
		t.Run(test, func(t *testing.T) {
			if !ng.IsSupportedReference(test) {
				t.Errorf("IsSupportedReference(%v) = false, want true", test)
			}
		})
	}

	unsupportedTests := []string{
		"simple string",
		"k8s.io/client-go/pkg/foo",
		"github.com/google/secret/cmd/sauce",
		path.Join("vendor", importpath, "pkg", "foo"),
	}

	for _, test := range unsupportedTests {
		t.Run(test, func(t *testing.T) {
			if ng.IsSupportedReference(test) {
				t.Errorf("IsSupportedReference(%v) = true, want false", test)
			}
		})
	}
}

// A helper method we use to substitute for the default "build" method.
func writeTempFile(s string) (string, error) {
	file, err := ioutil.TempFile(os.TempDir(), "out")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := file.WriteString(s); err != nil {
		return "", err
	}
	return file.Name(), nil
}

func TestGoBuild(t *testing.T) {
	baseLayers := int64(3)
	base, err := random.Image(1024, baseLayers)
	if err != nil {
		t.Fatalf("random.Image() = %v", err)
	}
	importpath := "github.com/google/go-containerregistry"
	tc := testContext{
		gopath:  "/go",
		workdir: "/go/src/" + importpath,
	}
	tc.Enter(t)
	defer tc.Exit(t)

	creationTime := func() (*v1.Time, error) {
		return &v1.Time{time.Unix(5000, 0)}, nil
	}

	ng, err := NewGo(
		Options{
			GetCreationTime: creationTime,
			GetBase: func(string) (v1.Image, error) {
				return base, nil
			},
		})
	if err != nil {
		t.Fatalf("NewGo() = %v", err)
	}
	ng.(*gobuild).build = writeTempFile

	img, err := ng.Build(path.Join(importpath, "cmd", "crane"))
	if err != nil {
		t.Errorf("Build() = %v", err)
	}

	ls, err := img.Layers()
	if err != nil {
		t.Errorf("Layers() = %v", err)
	}

	// Check that we have the expected number of layers.
	t.Run("check layer count", func(t *testing.T) {
		if got, want := int64(len(ls)), baseLayers+1; got != want {
			t.Fatalf("len(Layers()) = %v, want %v", got, want)
		}
	})

	// While we have a randomized base image, the application layer should be completely deterministic.
	// Check that when given fixed build outputs we get a fixed layer hash.
	t.Run("check determinism", func(t *testing.T) {
		expectedHash := v1.Hash{
			Algorithm: "sha256",
			Hex:       "b0780391d7e0cc1f43dc4531e73ee7493309ce446c2eeb9afe8866b623fcf3c2",
		}
		appLayer := ls[baseLayers]

		if got, err := appLayer.Digest(); err != nil {
			t.Errorf("Digest() = %v", err)
		} else if got != expectedHash {
			t.Errorf("Digest() = %v, want %v", got, expectedHash)
		}
	})

	// Check that the entrypoint of the image is configured to invoke our Go application
	t.Run("check entrypoint", func(t *testing.T) {
		cfg, err := img.ConfigFile()
		if err != nil {
			t.Errorf("ConfigFile() = %v", err)
		}
		entrypoint := cfg.Config.Entrypoint
		if got, want := len(entrypoint), 1; got != want {
			t.Errorf("len(entrypoint) = %v, want %v", got, want)
		}

		if got, want := entrypoint[0], appPath; got != want {
			t.Errorf("entrypoint = %v, want %v", got, want)
		}
	})

	t.Run("check creation time", func(t *testing.T) {
		cfg, err := img.ConfigFile()
		if err != nil {
			t.Errorf("ConfigFile() = %v", err)
		}

		actual := cfg.Created
		want, err := creationTime()
		if err != nil {
			t.Errorf("CreationTime() = %v", err)
		}

		if actual.Time != want.Time {
			t.Errorf("created = %v, want %v", actual, want)
		}
	})
}
