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
	"log"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/pkg/ko/build"
	"github.com/google/go-containerregistry/pkg/ko/publish"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
)

func publishImages(importpaths []string, lo *LocalOptions) {
	b, err := build.NewGo(gobuildOptions())
	if err != nil {
		log.Fatalf("error creating go builder: %v", err)
	}
	for _, importpath := range importpaths {
		img, err := b.Build(importpath)
		if err != nil {
			log.Fatalf("error building %q: %v", importpath, err)
		}
		var pub publish.Interface
		repoName := os.Getenv("KO_DOCKER_REPO")
		if lo.Local || repoName == publish.LocalDomain {
			pub = publish.NewDaemon(daemon.WriteOptions{})
		} else {
			repo, err := name.NewRepository(repoName, name.WeakValidation)
			if err != nil {
				log.Fatalf("the environment variable KO_DOCKER_REPO must be set to a valid docker repository, got %v", err)
			}
			pub = publish.NewDefault(repo, http.DefaultTransport)
		}
		if _, err := pub.Publish(img, importpath); err != nil {
			log.Fatalf("error publishing %s: %v", importpath, err)
		}
	}
}
