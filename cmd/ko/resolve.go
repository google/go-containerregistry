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

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/google/go-containerregistry/ko/build"
	"github.com/google/go-containerregistry/ko/publish"
	"github.com/google/go-containerregistry/ko/resolve"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1/remote"
)

func gobuildOptions() build.Options {
	return build.Options{
		GetBase: GetBaseImage,
	}
}

func resolveFilesToWriter(fo *FilenameOptions, out io.Writer) {
	fs, err := enumerateFiles(fo)
	if err != nil {
		log.Fatalf("error enumerating files: %v", err)
	}

	opt := gobuildOptions()
	var sm sync.Map
	wg := sync.WaitGroup{}
	for _, f := range fs {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			rc := resolveFile(f, opt)
			sm.Store(f, rc)
		}(f)
	}
	// Wait for all of the go routines to complete.
	wg.Wait()
	for _, f := range fs {
		iface, ok := sm.Load(f)
		if !ok {
			log.Fatalf("missing file in resolved map: %v", f)
		}
		rc, ok := iface.(io.ReadCloser)
		if !ok {
			log.Fatalf("unsupported type in sync.Map's value: %T", iface)
		}
		// Our sole output should be the resolved yamls
		out.Write([]byte("---\n"))
		if _, err := io.Copy(out, rc); err != nil {
			log.Fatalf("Error writing resolved output of %q: %v", f, err)
		}
		if err := rc.Close(); err != nil {
			log.Fatalf("Error closing resolved output of %q: %v", f, err)
		}
	}
}

type errReadCloser struct {
	err error
}

func (e errReadCloser) Read([]byte) (int, error) { return 0, e.err }
func (e errReadCloser) Close() error             { return e.err }

func resolveFile(fn string, opt build.Options) io.ReadCloser {
	repoName := os.Getenv("KO_DOCKER_REPO")
	repo, err := name.NewRepository(repoName, name.WeakValidation)
	if err != nil {
		return errReadCloser{fmt.Errorf("the environment variable KO_DOCKER_REPO must be set to a valid docker repository, got %v", err)}
	}

	f, err := os.Open(fn)
	if err != nil {
		return errReadCloser{err}
	}
	defer f.Close()

	publisher := publish.NewDefault(repo, http.DefaultTransport, remote.WriteOptions{
		MountPaths: GetMountPaths(),
	})
	builder, err := build.NewGo(opt)
	if err != nil {
		return errReadCloser{err}
	}

	// TODO(mattmoor): To better approximate Bazel, we should collect the importpath references
	// in advance, trigger builds, and then do a second pass to finalize each of the configs.
	return resolve.ImageReferences(f, builder, publisher)
}
