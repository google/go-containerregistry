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

package remote

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

type tags struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// List calls /tags/list for the given repository, returning the list of tags
// in the "tags" property.
func List(repo name.Repository, options ...Option) ([]string, error) {
	o, err := makeOptions(repo, options...)
	if err != nil {
		return nil, err
	}
	scopes := []string{repo.Scope(transport.PullScope)}
	tr, err := transport.New(repo.Registry, o.auth, o.transport, scopes)
	if err != nil {
		return nil, err
	}

	uri := &url.URL{
		Scheme: repo.Registry.Scheme(),
		Host:   repo.Registry.RegistryStr(),
		Path:   fmt.Sprintf("/v2/%s/tags/list", repo.RepositoryStr()),
	}

	client := http.Client{Transport: tr}
	tagList := []string{}
	parsed := tags{}

	// get responses until there is no next page
	for {
		resp, err := client.Get(uri.String())
		if err != nil {
			return nil, err
		}

		if err := transport.CheckError(resp, http.StatusOK); err != nil {
			return nil, err
		}

		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			return nil, err
		}

		if err := resp.Body.Close(); err != nil {
			return nil, err
		}

		tagList = append(tagList, parsed.Tags...)

		uri, err = getNextPageURL(resp)
		if err != nil {
			return nil, err
		}
		// no next page
		if uri == nil {
			break
		}
	}

	return tagList, nil
}

// getNextPageURL checks if there is a Link header in a http.Response which
// contains a link to the next page. If yes it returns the url.URL of the next
// page otherwise it returns nil.
func getNextPageURL(resp *http.Response) (*url.URL, error) {
	link := resp.Header.Get("Link")
	if link == "" {
		return nil, nil
	}

	if link[0] != '<' {
		return nil, fmt.Errorf("failed to parse link header: missing '<' in: %s", link)
	}

	end := strings.Index(link, ">")
	if end == -1 {
		return nil, fmt.Errorf("failed to parse link header: missing '>' in: %s", link)
	}
	link = link[1:end]

	linkURL, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	linkURL = resp.Request.URL.ResolveReference(linkURL)
	return linkURL, nil
}
