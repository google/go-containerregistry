// Copyright 2022 Google LLC All Rights Reserved.
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
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAuthConfigMarshalJSON(t *testing.T) {
	cases := []struct {
		name   string
		config AuthConfig
		json   string
	}{{
		name: "auth field is calculated",
		config: AuthConfig{
			Username:      "user",
			Password:      "pass",
			IdentityToken: "id",
			RegistryToken: "reg",
		},
		json: `{"username":"user","password":"pass","auth":"dXNlcjpwYXNz","identitytoken":"id","registrytoken":"reg"}`,
	}, {
		name: "auth field replaced",
		config: AuthConfig{
			Username:      "user",
			Password:      "pass",
			Auth:          "blah",
			IdentityToken: "id",
			RegistryToken: "reg",
		},
		json: `{"username":"user","password":"pass","auth":"dXNlcjpwYXNz","identitytoken":"id","registrytoken":"reg"}`,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bytes, err := json.Marshal(&tc.config)

			if err != nil {
				t.Fatal("Marshal() =", err)
			}

			if diff := cmp.Diff(tc.json, string(bytes)); diff != "" {
				t.Error("json output diff (-want, +got): ", diff)
			}
		})
	}
}

func TestAuthConfigUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		json string
		err  string
		want AuthConfig
	}{{
		name: "valid config no auth",
		json: `{
			"username": "user",
			"password": "pass",
			"identitytoken": "id",
			"registrytoken": "reg"
		}`,
		want: AuthConfig{
			// Auth value is set based on username and password
			Auth:          "dXNlcjpwYXNz",
			Username:      "user",
			Password:      "pass",
			IdentityToken: "id",
			RegistryToken: "reg",
		},
	}, {
		name: "bad json input",
		json: `{"username":true}`,
		err:  "json: cannot unmarshal",
	}, {
		name: "auth is base64",
		json: `{ "auth": "dXNlcjpwYXNz" }`, // user:pass
		want: AuthConfig{
			Username: "user",
			Password: "pass",
			Auth:     "dXNlcjpwYXNz",
		},
	}, {
		name: "auth field overrides others",
		json: `{ "auth": "dXNlcjpwYXNz", "username":"foo", "password":"bar" }`, // user:pass
		want: AuthConfig{
			Username: "user",
			Password: "pass",
			Auth:     "dXNlcjpwYXNz",
		},
	}, {
		name: "auth is base64 padded",
		json: `{ "auth": "dXNlcjpwYXNzd29yZA==" }`, // user:password
		want: AuthConfig{
			Username: "user",
			Password: "password",
			Auth:     "dXNlcjpwYXNzd29yZA==",
		},
	}, {
		name: "auth is not base64",
		json: `{ "auth": "bad-auth-bad" }`,
		err:  "unable to decode auth field",
	}, {
		name: "decoded auth is not valid",
		json: `{ "auth": "Zm9vYmFy" }`,
		err:  "unable to decode auth field: must be formatted as base64(username:password)",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got AuthConfig
			err := json.Unmarshal([]byte(tc.json), &got)
			if tc.err != "" && err == nil {
				t.Fatal("no error occurred expected:", tc.err)
			} else if tc.err != "" && err != nil {
				if !strings.HasPrefix(err.Error(), tc.err) {
					t.Fatalf("expected err %q to have prefix %q", err, tc.err)
				}
				return
			}

			if err != nil {
				t.Fatal("Unmarshal()=", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatal("unexpected diff (-want, +got)\n", diff)
			}
		})
	}
}
