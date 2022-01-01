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

package google

import gauth "github.com/google/go-containerregistry/pkg/authn/google"

var (
	// Keychain exports an instance of the google Keychain.
	//
	// Deprecated: Use pkg/authn/google.Keychain instead.
	Keychain = gauth.Keychain

	// GetGcloudCmd is exposed so we can test this.
	//
	// Deprecated: Use pkg/authn/google.GetGcloudCmd instead.
	GetGcloudCmd = gauth.GetGcloudCmd

	// NewEnvAuthenticator returns an authn.Authenticator that generates
	// access tokens from the environment we're running in.
	//
	// See: https://godoc.org/golang.org/x/oauth2/google#FindDefaultCredentials
	//
	// Deprecated: Use pkg/authn/google.NewEnvAuthenticator instead.
	NewEnvAuthenticator = gauth.NewEnvAuthenticator

	// NewGcloudAuthenticator returns an oauth2.TokenSource that generates
	// access tokens by shelling out to the gcloud sdk.
	//
	// Deprecated: Use pkg/authn/google.NewGcloudAuthenticator instead.
	NewGcloudAuthenticator = gauth.NewGcloudAuthenticator

	// NewJSONKeyAuthenticator returns a Basic authenticator which uses
	// Service Account as a way of authenticating with Google Container
	// Registry.
	//
	// See: https://cloud.google.com/container-registry/docs/advanced-authentication#json_key_file
	//
	// Deprecated: Use pkg/authn/google.NewJSONKeyAuthenticator instead.
	NewJSONKeyAuthenticator = gauth.NewJSONKeyAuthenticator

	// NewTokenAuthenticator returns an oauth2.TokenSource that generates
	// access tokens by using the Google SDK to produce JWT tokens from a
	// Service Account.
	//
	// See: https://godoc.org/golang.org/x/oauth2/google#JWTAccessTokenSourceFromJSON
	//
	// Deprecated: Use pkg/authn/google.NewTokenAuthenticator instead.
	NewTokenAuthenticator = gauth.NewTokenAuthenticator

	// NewTokenSourceAuthenticator converts an oauth2.TokenSource into an
	// authn.Authenticator.
	//
	// Deprecated: Use pkg/authn/google.NewTokenSourceAuthenticator instead.
	NewTokenSourceAuthenticator = gauth.NewTokenSourceAuthenticator
)
