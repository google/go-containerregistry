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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// TagDetails keeps various details related to a tag which
// are returned form the remote registry
type TagDetails struct {
	Sha            string    `json:"sha"`
	ImageSizeBytes string    `json:"image_size_bytes"`
	Tags           []string  `json:"tags"`
	CreateTime     time.Time `json:"create_time"`
	UploadTime     time.Time `json:"upload_time"`
}

type tags struct {
	Manifest map[string]tagDetails `json:"manifest,omitempty"`
	Name     string                `json:"name"`
	Tags     []string              `json:"tags"`
}

type tagDetails struct {
	ImageSizeBytes string   `json:"imageSizeBytes,omitempty"`
	Tag            []string `json:"tag,omitempty"`
	TimeCreatedMs  string   `json:"timeCreatedMs,omitempty"`
	TimeUploadedMs string   `json:"timeUploadedMs,omitempty"`
}

// createTagDetailsList builds a TagDetails list from the manifest returned by
// the remote repository
func createTagDetailsList(manifest map[string]tagDetails) []TagDetails {
	tags := []TagDetails{}
	for sha, tag := range manifest {
		// Skip empty tags
		if len(tag.Tag) == 0 {
			continue
		}
		createTimeMs, err := strconv.ParseInt(tag.TimeCreatedMs, 10, 64)
		if err != nil {
			continue
		}
		uploadTimeMs, err := strconv.ParseInt(tag.TimeUploadedMs, 10, 64)
		if err != nil {
			continue
		}
		tagDetails := TagDetails{
			Sha:            sha,
			ImageSizeBytes: tag.ImageSizeBytes,
			Tags:           tag.Tag,
			CreateTime:     time.Unix(createTimeMs/1000, 0),
			UploadTime:     time.Unix(uploadTimeMs/1000, 0),
		}
		tags = append(tags, tagDetails)
	}
	return tags
}

// List wraps ListWithContext using the background context.
func List(repo name.Repository, options ...Option) ([]string, error) {
	return ListWithContext(context.Background(), repo, options...)
}

// ListWithContext calls /tags/list for the given repository, returning the list of tags
// in the "tags" property.
func ListWithContext(ctx context.Context, repo name.Repository, options ...Option) ([]string, error) {
	tags, _, err := listWithContext(ctx, repo, options...)
	return tags, err
}

// ListDetails wraps ListDeatilsWithContext using the background context.
func ListDetails(repo name.Repository, options ...Option) ([]string, []TagDetails, error) {
	return ListDetailsWithContext(context.Background(), repo, options...)
}

// ListDetailsWithContext calls /tags/list for the given repository, returning the list of tags
// in the "tags" property along with a list of tag details available in the "manifest" property
func ListDetailsWithContext(ctx context.Context, repo name.Repository, options ...Option) ([]string, []TagDetails, error) {
	return listWithContext(ctx, repo, options...)
}

// ListWithContext calls /tags/list for the given repository, returning the list of tags
// in the "tags" property.
func listWithContext(ctx context.Context, repo name.Repository, options ...Option) ([]string, []TagDetails, error) {
	o, err := makeOptions(repo, options...)
	if err != nil {
		return nil, nil, err
	}
	scopes := []string{repo.Scope(transport.PullScope)}
	tr, err := transport.New(repo.Registry, o.auth, o.transport, scopes)
	if err != nil {
		return nil, nil, err
	}

	uri := &url.URL{
		Scheme: repo.Registry.Scheme(),
		Host:   repo.Registry.RegistryStr(),
		Path:   fmt.Sprintf("/v2/%s/tags/list", repo.RepositoryStr()),
		// ECR returns an error if n > 1000:
		// https://github.com/google/go-containerregistry/issues/681
		RawQuery: "n=1000",
	}

	client := http.Client{Transport: tr}
	tagList := []string{}
	tagDetailsList := []TagDetails{}

	// get responses until there is no next page
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		req, err := http.NewRequest("GET", uri.String(), nil)
		if err != nil {
			return nil, nil, err
		}
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}

		if err := transport.CheckError(resp, http.StatusOK); err != nil {
			return nil, nil, err
		}

		tags, tdList, err := decodeReponse(resp.Body)
		if err != nil {
			return nil, nil, err
		}
		tagList = append(tagList, tags...)
		tagDetailsList = append(tagDetailsList, tdList...)

		if err := resp.Body.Close(); err != nil {
			return nil, nil, err
		}

		uri, err = getNextPageURL(resp)
		if err != nil {
			return nil, nil, err
		}
		// no next page
		if uri == nil {
			break
		}
	}

	return tagList, tagDetailsList, nil
}

// decodeReponse parse the tags details from response
func decodeReponse(resp io.ReadCloser) ([]string, []TagDetails, error) {
	parsed := tags{}
	if err := json.NewDecoder(resp).Decode(&parsed); err != nil {
		return []string{}, []TagDetails{}, err
	}
	return parsed.Tags, createTagDetailsList(parsed.Manifest), nil
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
	if resp.Request == nil || resp.Request.URL == nil {
		return nil, nil
	}
	linkURL = resp.Request.URL.ResolveReference(linkURL)
	return linkURL, nil
}
