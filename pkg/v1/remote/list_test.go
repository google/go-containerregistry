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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
)

func TestList(t *testing.T) {
	cases := []struct {
		name         string
		responseBody []byte
		wantErr      bool
		wantTags     []string
	}{{
		name:         "success",
		responseBody: []byte(`{"tags":["foo","bar"]}`),
		wantErr:      false,
		wantTags:     []string{"foo", "bar"},
	}, {
		name:         "not json",
		responseBody: []byte("notjson"),
		wantErr:      true,
	}}

	repoName := "ubuntu"

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tagsPath := fmt.Sprintf("/v2/%s/tags/list", repoName)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/":
					w.WriteHeader(http.StatusOK)
				case tagsPath:
					if r.Method != http.MethodGet {
						t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
					}

					w.Write(tc.responseBody)
				default:
					t.Fatalf("Unexpected path: %v", r.URL.Path)
				}
			}))
			defer server.Close()
			u, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("url.Parse(%v) = %v", server.URL, err)
			}

			repo, err := name.NewRepository(fmt.Sprintf("%s/%s", u.Host, repoName), name.WeakValidation)
			if err != nil {
				t.Fatalf("name.NewRepository(%v) = %v", repoName, err)
			}

			tags, err := List(repo)
			if (err != nil) != tc.wantErr {
				t.Errorf("List() wrong error: %v, want %v: %v\n", (err != nil), tc.wantErr, err)
			}

			if diff := cmp.Diff(tc.wantTags, tags); diff != "" {
				t.Errorf("List() wrong tags (-want +got) = %s", diff)
			}
		})
	}
}

func TestCancelledList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repoName := "doesnotmatter"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	repo, err := name.NewRepository(fmt.Sprintf("%s/%s", u.Host, repoName), name.WeakValidation)
	if err != nil {
		t.Fatalf("name.NewRepository(%v) = %v", repoName, err)
	}

	_, err = ListWithContext(ctx, repo)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Errorf(`unexpected error; want "context canceled", got %v`, err)
	}
}

func makeResp(hdr string) *http.Response {
	return &http.Response{
		Header: http.Header{
			"Link": []string{hdr},
		},
	}
}

func TestGetNextPageURL(t *testing.T) {
	for _, hdr := range []string{
		"",
		"<",
		"><",
		"<>",
		fmt.Sprintf("<%c>", 0x7f), // makes url.Parse fail
	} {
		u, err := getNextPageURL(makeResp(hdr))
		if err == nil && u != nil {
			t.Errorf("Expected err, got %+v", u)
		}
	}

	good := &http.Response{
		Header: http.Header{
			"Link": []string{"<example.com>"},
		},
		Request: &http.Request{
			URL: &url.URL{
				Scheme: "https",
			},
		},
	}
	u, err := getNextPageURL(good)
	if err != nil {
		t.Fatal(err)
	}

	if u.Scheme != "https" {
		t.Errorf("expected scheme to match request, got %s", u.Scheme)
	}
}
