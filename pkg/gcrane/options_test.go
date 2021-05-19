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

package gcrane

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
)

func TestOptions(t *testing.T) {
	o := makeOptions()
	if len(o.remote) != 1 {
		t.Errorf("remote should default to Keychain")
	}
	if len(o.crane) != 1 {
		t.Errorf("crane should default to Keychain")
	}
	if len(o.google) != 1 {
		t.Errorf("google should default to Keychain")
	}

	o = makeOptions(WithAuth(authn.Anonymous), WithKeychain(authn.DefaultKeychain))
	if len(o.remote) != 1 {
		t.Errorf("WithKeychain should replace remote[0]")
	}
	if len(o.crane) != 1 {
		t.Errorf("WithKeychain should replace crane[0]")
	}
	if len(o.google) != 1 {
		t.Errorf("WithKeychain should replace google[0]")
	}

	o = makeOptions(WithTransport(http.DefaultTransport), WithUserAgent("hi"), WithContext(context.TODO()))
	if len(o.remote) != 4 {
		t.Errorf("wrong number of options: %d", len(o.remote))
	}
	if len(o.crane) != 4 {
		t.Errorf("wrong number of options: %d", len(o.crane))
	}
	if len(o.google) != 4 {
		t.Errorf("wrong number of options: %d", len(o.google))
	}
}
