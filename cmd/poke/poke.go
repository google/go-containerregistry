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
	"log"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/remote"
)

func dispatch(cmd string, i v1.Image) (string, error) {
	switch cmd {
	case "config":
		config, err := i.RawConfigFile()
		if err != nil {
			return "", err
		}
		return string(config), nil
	case "digest":
		digest, err := i.Digest()
		if err != nil {
			return "", err
		}
		return digest.String(), nil
	case "manifest":
		manifest, err := i.RawManifest()
		if err != nil {
			return "", err
		}
		return string(manifest), nil
	default:
		return "", fmt.Errorf("unexpected subcommand: %s", cmd)
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Errorf("Usage:\n")
		fmt.Errorf(" %s config <reference>\n", os.Args[0])
		fmt.Errorf(" %s digest <reference>\n", os.Args[0])
		fmt.Errorf(" %s manifest <reference>\n", os.Args[0])
		os.Exit(1)
	}

	ref, err := name.ParseReference(os.Args[2], name.WeakValidation)
	if err != nil {
		log.Fatalln(err)
	}
	auth, err := authn.DefaultKeychain.Resolve(ref.Context().Registry)
	if err != nil {
		log.Fatalln(err)
	}
	i, err := remote.Image(ref, auth, http.DefaultTransport)
	if err != nil {
		log.Fatalln(err)
	}
	out, err := dispatch(os.Args[1], i)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(out)
}
