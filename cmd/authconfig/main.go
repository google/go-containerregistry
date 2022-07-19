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
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
)

var server = flag.String("server", "", "image registry server name")

var (
	amazonKeychain authn.Keychain = authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(ioutil.Discard)))
	azureKeychain  authn.Keychain = authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper())
)

func main() {
	flag.Parse()

	keychain := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		amazonKeychain,
		azureKeychain,
	)

	resource, err := name.NewRegistry(*server)
	if err != nil {
		log.Fatalf("failed to parse the server: %v", err)
	}

	authenticator, err := keychain.Resolve(resource)
	if err != nil {
		log.Fatalf("failed to retrieve credential: %v", err)
	}

	authConfig, err := authenticator.Authorization()
	if err != nil {
		log.Fatalf("failed to retrieve credential: %v", err)
	}

	json, err := authConfig.MarshalJSON()
	if err != nil {
		log.Fatalf("failed to retrieve credential: %v", err)
	}

	os.Stdout.Write(json)
}
