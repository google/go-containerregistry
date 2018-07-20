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

// WalkFunc is the type of the function called for each repository visited by
// Walk. This implements a simliar API to filepath.Walk.
//
// The repo argument contains the argument to Walk as a prefix; that is, if Walk
// is called with "gcr.io/foo", which is a repository containing the repository
// "bar", the walk function will be called with argument "gcr.io/foo/bar".
// The tags and error arguments are the result of calling List on repo.
//
// TODO: Do we want a SkipDir error, as in filepath.WalkFunc?
type WalkFunc func(repo name.Repository, tags *Tags, err error) error

func walk(repo name.Repository, auth authn.Authenticator, t http.RoundTripper, tags *Tags, walkFn WalkFunc) error {
	if tags == nil {
		// This shouldn't happen.
		return fmt.Errorf("tags nil for %q", repo)
	}

	if err := walkFn(repo, tags, nil); err != nil {
		return err
	}

	for _, path := range tags.Children {
		child, err := name.NewRepository(fmt.Sprintf("%s/%s", repo, path), name.StrictValidation)
		if err != nil {
			// We don't expect this ever, so don't pass it through to walkFn.
			return fmt.Errorf("unexpected path failure: %v", err)
		}

		childTags, err := List(child, auth, t)
		if err != nil {
			if err := walkFn(repo, nil, err); err != nil {
				return err
			}
		} else {
			if err := walk(child, auth, t, childTags, walkFn); err != nil {
				return err
			}
		}
	}

	// We made it!
	return nil
}

// Walk recursively descends repositories, calling walkFn.
// TODO: Do we need a keychain since this can take a while?
func Walk(root name.Repository, auth authn.Authenticator, t http.RoundTripper, walkFn WalkFunc) error {
	tags, err := List(root, auth, t)
	if err != nil {
		return walkFn(root, nil, err)
	}

	return walk(root, auth, t, tags, walkFn)
}
