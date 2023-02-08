// Copyright 2023 Google LLC All Rights Reserved.
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

package crane

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestInsecureOptionTracking(t *testing.T) {
	want := true
	opts := GetOptions(Insecure)

	if got := opts.insecure; got != want {
		t.Errorf("got %t\nwant: %t", got, want)
	}
}

func TestTransportSetting(t *testing.T) {
	opts := GetOptions(WithTransport(remote.DefaultTransport))

	if opts.transport == nil {
		t.Error("expected crane transport to be set when user passes WithTransport")
	}
}

func TestInsecureTransport(t *testing.T) {
	want := true
	opts := GetOptions(Insecure)
	var transport *http.Transport
	var ok bool
	if transport, ok = opts.transport.(*http.Transport); !ok {
		t.Fatal("Unable to successfully assert default transport")
	}

	if transport.TLSClientConfig == nil {
		t.Fatal(errors.New("TLSClientConfig was nil and should be set"))
	}

	if got := transport.TLSClientConfig.InsecureSkipVerify; got != want {
		t.Errorf("got: %t\nwant: %t", got, want)
	}
}
