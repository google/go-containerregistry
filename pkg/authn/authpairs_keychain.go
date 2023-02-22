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
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/google/go-containerregistry/pkg/name"
)

// AuthPairs is a key-value map in the form TARGET-REPO:TARGET-DOCKER-CONFIG.
//
// Example:
// gcr.io/one/repo/name:/path/to/docker/config1
// gcr.io/two/repo/name:/path/to/docker/config2
type AuthPairs map[string]string

func (ap AuthPairs) Has(target Resource) bool {
	if ap == nil {
		return false
	}
	_, ok := ap[target.String()]
	return ok
}

func (ap AuthPairs) Dir(target Resource) string {
	if ap == nil {
		return ""
	}
	return ap[target.String()]
}

func ParseAuthPair(authPairs AuthPairs, authPair string) (AuthPairs, error) {
	if authPairs == nil {
		authPairs = make(map[string]string)
	}

	parts := strings.SplitN(authPair, ":", 2)
	ref, err := name.ParseReference(parts[0])
	if err != nil {
		return authPairs, err
	}

	authPairs[ref.Context().String()] = parts[1]

	return authPairs, nil
}

type authPairsKeychain struct {
	mu        sync.Mutex
	cache     map[string]Authenticator
	authPairs AuthPairs
}

var _ Keychain = (*authPairsKeychain)(nil)

func NewAuthPairsKeychain(authPairs AuthPairs) Keychain {
	return &authPairsKeychain{
		cache:     make(map[string]Authenticator),
		authPairs: authPairs,
	}
}

func (mk *authPairsKeychain) Resolve(target Resource) (Authenticator, error) {
	mk.mu.Lock()
	defer mk.mu.Unlock()

	if authenticator, ok := mk.cache[target.String()]; ok {
		return authenticator, nil
	}

	if !mk.authPairs.Has(target) {
		return DefaultKeychain.Resolve(target)
	}

	dir := mk.authPairs.Dir(target)

	var (
		cf  *configfile.ConfigFile
		err error
	)

	// Check for Docker config file first, then for Podman auth file
	if fileExists(filepath.Join(dir, config.ConfigFileName)) {
		cf, err = config.Load(dir)
		if err != nil {
			return nil, err
		}
	} else if podmanAuthFile := filepath.Join(dir, "auth.json"); fileExists(podmanAuthFile) {
		cf, err = config.Load(dir)
		f, err := os.Open(podmanAuthFile)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		cf, err = config.LoadFromReader(f)
		if err != nil {
			return nil, err
		}
	}

	if cf == nil {
		return Anonymous, nil
	}

	auth, err := getAuthenticator(cf, target)
	if err != nil {
		return nil, err
	}

	mk.cache[target.String()] = auth

	return auth, nil
}
