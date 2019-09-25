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
	"github.com/docker/cli/cli/config"
)

// Resource represents a registry or repository that can be authenticated against.
type Resource interface {
	// String returns the full string representation of the target, e.g.
	// gcr.io/my-project or just gcr.io.
	String() string

	// RegistryStr returns just the registry portion of the target, e.g. for
	// gcr.io/my-project, this should just return gcr.io. This is needed to
	// pull out an appropriate hostname.
	RegistryStr() string
}

// Keychain is an interface for resolving an image reference to a credential.
type Keychain interface {
	// Resolve looks up the most appropriate credential for the specified target.
	Resolve(Resource) (Authenticator, error)
}

// defaultKeychain implements Keychain with the semantics of the standard Docker
// credential keychain.
type defaultKeychain struct{}

var (
	// Export an instance of the default keychain.
	DefaultKeychain Keychain = &defaultKeychain{}
)

// Resolve implements Keychain.
func (dk *defaultKeychain) Resolve(target Resource) (Authenticator, error) {
	cf, err := config.Load("")
	if err != nil {
		return nil, err
	}

	cfg, err := cf.GetAuthConfig(target.RegistryStr())
	if err != nil {
		return nil, err
	}

	// TODO: remove this for sure
	// b, err := json.Marshal(cfg)
	// if err != nil {
	// 	return nil, err
	// }
	// log.Println(string(b))

	// TODO: Do we need this?
	if cfg.Username == "" && cfg.Auth == "" && cfg.IdentityToken == "" && cfg.RegistryToken == "" {
		return Anonymous, nil
	}
	return &Basic{Username: cfg.Username, Password: cfg.Password}, nil
}
