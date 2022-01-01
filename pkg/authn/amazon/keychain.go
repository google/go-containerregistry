// Copyright 2021 Google LLC All Rights Reserved.
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

package amazon

import (
	"github.com/awslabs/amazon-ecr-credential-helper/ecr-login/api"
	"github.com/google/go-containerregistry/pkg/authn"
)

// Keychain exports an instance of the amazon Keychain.
var Keychain authn.Keychain = amazonKeychain{}

type amazonKeychain struct{}

// Resolve implements authn.Keychain a la docker-credential-ecr-login.
func (amazonKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	reg, err := api.ExtractRegistry(target.String())
	if err != nil {
		return authn.Anonymous, nil
	}

	cf := api.DefaultClientFactory{}
	var client api.Client
	if reg.FIPS {
		client, err = cf.NewClientWithFipsEndpoint(reg.Region)
		if err != nil {
			return authn.Anonymous, nil
		}
	} else {
		client = cf.NewClientFromRegion(reg.Region)
	}

	auth, err := client.GetCredentials(target.String())
	if err != nil {
		return authn.Anonymous, nil
	}
	return &authn.Basic{
		Username: auth.Username,
		Password: auth.Password,
	}, nil
}
