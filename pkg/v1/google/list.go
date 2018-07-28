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

package google

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

type rawManifestInfo struct {
	Size      string   `json:"imageSizeBytes"`
	MediaType string   `json:"mediaType"`
	Created   string   `json:"timeCreatedMs"`
	Uploaded  string   `json:"timeUploadedMs"`
	Tags      []string `json:"tag"`
}

type ManifestInfo struct {
	Size      uint64    `json:"imageSizeBytes"`
	MediaType string    `json:"mediaType"`
	Created   time.Time `json:"timeCreatedMs"`
	Uploaded  time.Time `json:"timeUploadedMs"`
	Tags      []string  `json:"tag"`
}

func fromUnixMs(ms int64) time.Time {
	sec := ms / 1000
	ns := (ms % 1000) * 1000000
	return time.Unix(sec, ns)
}

// UnmarshalJSON implements json.Unmarshaler
func (m *ManifestInfo) UnmarshalJSON(data []byte) error {
	raw := rawManifestInfo{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.Size != "" {
		size, err := strconv.ParseUint(string(raw.Size), 10, 64)
		if err != nil {
			return err
		}
		m.Size = size
	}

	if raw.Created != "" {
		created, err := strconv.ParseInt(string(raw.Created), 10, 64)
		if err != nil {
			return err
		}
		m.Created = fromUnixMs(created)
	}

	if raw.Uploaded != "" {
		uploaded, err := strconv.ParseInt(string(raw.Uploaded), 10, 64)
		if err != nil {
			return err
		}
		m.Uploaded = fromUnixMs(uploaded)
	}

	m.MediaType = raw.MediaType
	m.Tags = raw.Tags

	return nil
}

type Tags struct {
	Children  []string                `json:"child"`
	Manifests map[string]ManifestInfo `json:"manifest"`
	Name      string                  `json:"name"`
	Tags      []string                `json:"tags"`
}

func List(repo name.Repository, auth authn.Authenticator, t http.RoundTripper) (*Tags, error) {
	scopes := []string{repo.Scope(transport.PullScope)}
	tr, err := transport.New(repo.Registry, auth, t, scopes)
	if err != nil {
		return nil, err
	}

	uri := url.URL{
		Scheme: repo.Registry.Scheme(),
		Host:   repo.Registry.RegistryStr(),
		Path:   fmt.Sprintf("/v2/%s/tags/list", repo.RepositoryStr()),
	}

	client := http.Client{Transport: tr}
	resp, err := client.Get(uri.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := remote.CheckError(resp, http.StatusOK); err != nil {
		return nil, err
	}

	tags := Tags{}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}

	return &tags, nil
}
