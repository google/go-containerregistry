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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/authn"
)

type bearerTransport struct {
	inner   http.RoundTripper
	basic   authn.Authenticator
	bearer  *authn.Bearer
	realm   string
	service string
	scope   string
}

var _ http.RoundTripper = (*bearerTransport)(nil)

// RoundTrip implements http.RoundTripper
func (bt *bearerTransport) RoundTrip(in *http.Request) (*http.Response, error) {
	hdr, err := bt.bearer.Authorization()
	if err != nil {
		return nil, err
	}
	in.Header.Set("Authorization", hdr)
	in.Header.Set("User-Agent", transportName)

	// TODO(mattmoor): On 401s perform a single refresh() and retry.
	return bt.inner.RoundTrip(in)
}

func (bt *bearerTransport) refresh() error {
	b := &basicTransport{inner: bt.inner, auth: bt.basic}
	client := http.Client{Transport: b}

	resp, err := client.Get(fmt.Sprintf("%s?%s",
		bt.realm, url.Values{
			"scope":   []string{bt.scope},
			"service": []string{bt.service},
		}.Encode()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Parse the response into a Bearer authenticator
	bearer := &authn.Bearer{}
	if err := json.Unmarshal(content, bearer); err != nil {
		return err
	}
	// Replace our old bearer authenticator (if we had one) with our newly refreshed authenticator.
	bt.bearer = bearer
	return nil
}
