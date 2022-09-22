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

type multiKeychain struct {
	keychains []Keychain
}

// Assert that our multi-keychain implements Keychain.
var _ (Keychain) = (*multiKeychain)(nil)

// NewMultiKeychain composes a list of keychains into one new keychain.
func NewMultiKeychain(kcs ...Keychain) Keychain {
	return &multiKeychain{keychains: kcs}
}

// Resolve implements Keychain.
func (mk *multiKeychain) Resolve(target Resource) (Authenticator, error) {
	var auths []Authenticator
	for _, kc := range mk.keychains {
		if kcMany, ok := kc.(KeychainMany); ok {
			manyAuths, err := kcMany.ResolveMany(target)
			if err != nil {
				return nil, err
			}
			noneAnonymousAuths := filterAnonymousAuthenticator(manyAuths...)
			auths = append(auths, noneAnonymousAuths...)
		} else {
			singleAuth, err := kc.Resolve(target)
			if err != nil {
				return nil, err
			}
			noneAnonymousAuths := filterAnonymousAuthenticator(singleAuth)
			auths = append(auths, noneAnonymousAuths...)
		}
	}
	if len(auths) == 0 {
		return Anonymous, nil
	}
	return &multiAuthenticator{auths: auths}, nil
}

// filterAnonymousAuthenticator filter Authenticators which is not Anonymous.
func filterAnonymousAuthenticator(auths ...Authenticator) []Authenticator {
	var noneAnonymousAuths []Authenticator
	for _, v := range auths {
		if v != Anonymous {
			noneAnonymousAuths = append(noneAnonymousAuths, v)
		}
	}

	return noneAnonymousAuths
}

type multiAuthenticator struct {
	auths []Authenticator
}

// FromConfigs returns an Authenticator that returns the given multiple AuthConfig.
func FromConfigs(cfgs []AuthConfig) Authenticator {
	multiAuth := multiAuthenticator{auths: []Authenticator{}}
	for _, cfg := range cfgs {
		auth := FromConfig(cfg)
		multiAuth.auths = append(multiAuth.auths, auth)
	}
	return &multiAuth
}

// Authorization implements Authenticator.
func (ma *multiAuthenticator) Authorization() (*AuthConfig, error) {
	auths, err := ma.Authorizations()
	if err != nil {
		return nil, err
	}
	return &auths[0], nil
}

// Authorizations implements get multiple Authenticator.
func (ma *multiAuthenticator) Authorizations() ([]AuthConfig, error) {
	auths := []AuthConfig{}
	for _, a := range ma.auths {
		a, err := a.Authorization()
		if err != nil {
			return nil, err
		}
		auths = append(auths, *a)
	}
	return auths, nil
}
