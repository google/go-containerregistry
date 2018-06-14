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

package transport

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
)

func TestSheme(t *testing.T) {
	tests := []struct {
		domain string
		scheme string
	}{{
		domain: "foo.svc.local:1234",
		scheme: "http",
	}, {
		domain: "127.0.0.1:1234",
		scheme: "http",
	}, {
		domain: "127.0.0.1",
		scheme: "http",
	}, {
		domain: "localhost:8080",
		scheme: "http",
	}, {
		domain: "gcr.io",
		scheme: "https",
	}, {
		domain: "index.docker.io",
		scheme: "https",
	}}

	for _, test := range tests {
		reg, err := name.NewRegistry(test.domain, name.WeakValidation)
		if err != nil {
			t.Errorf("NewRegistry(%s) = %v", test.domain, err)
		}
		if got, want := Scheme(reg), test.scheme; got != want {
			t.Errorf("scheme(%v); got %v, want %v", reg, got, want)
		}
	}
}
