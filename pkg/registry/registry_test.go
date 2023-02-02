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

package registry_test

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

const (
	weirdIndex = `{
  "manifests": [
	  {
			"digest":"sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			"mediaType":"application/vnd.oci.image.layer.nondistributable.v1.tar+gzip"
		},{
			"digest":"sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			"mediaType":"application/xml"
		},{
			"digest":"sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			"mediaType":"application/vnd.oci.image.manifest.v1+json"
		}
	]
}`
)

func sha256String(s string) string {
	h, _, _ := v1.SHA256(strings.NewReader(s))
	return h.Hex
}

func TestCalls(t *testing.T) {
	tcs := []struct {
		Description string

		// Request / setup
		URL           string
		Digests       map[string]string
		Manifests     map[string]string
		BlobStream    map[string]string
		RequestHeader map[string]string

		// Response
		Code   int
		Header map[string]string
		Method string
		Body   string // request body to send
		Want   string // response body to expect
	}{
		{
			Description: "/v2 returns 200",
			Method:      "GET",
			URL:         "/v2",
			Code:        http.StatusOK,
			Header:      map[string]string{"Docker-Distribution-API-Version": "registry/2.0"},
		},
		{
			Description: "/v2/ returns 200",
			Method:      "GET",
			URL:         "/v2/",
			Code:        http.StatusOK,
			Header:      map[string]string{"Docker-Distribution-API-Version": "registry/2.0"},
		},
		{
			Description: "/v2/bad returns 404",
			Method:      "GET",
			URL:         "/v2/bad",
			Code:        http.StatusNotFound,
			Header:      map[string]string{"Docker-Distribution-API-Version": "registry/2.0"},
		},
		{
			Description: "GET non existent blob",
			Method:      "GET",
			URL:         "/v2/foo/blobs/sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			Code:        http.StatusNotFound,
		},
		{
			Description: "HEAD non existent blob",
			Method:      "HEAD",
			URL:         "/v2/foo/blobs/sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			Code:        http.StatusNotFound,
		},
		{
			Description: "GET bad digest",
			Method:      "GET",
			URL:         "/v2/foo/blobs/sha256:asd",
			Code:        http.StatusBadRequest,
		},
		{
			Description: "HEAD bad digest",
			Method:      "HEAD",
			URL:         "/v2/foo/blobs/sha256:asd",
			Code:        http.StatusBadRequest,
		},
		{
			Description: "bad blob verb",
			Method:      "FOO",
			URL:         "/v2/foo/blobs/sha256:asd",
			Code:        http.StatusBadRequest,
		},
		{
			Description: "GET containerless blob",
			Digests:     map[string]string{"sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae": "foo"},
			Method:      "GET",
			URL:         "/v2/foo/blobs/sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			Code:        http.StatusOK,
			Header:      map[string]string{"Docker-Content-Digest": "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
			Want:        "foo",
		},
		{
			Description: "GET blob",
			Digests:     map[string]string{"sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae": "foo"},
			Method:      "GET",
			URL:         "/v2/foo/blobs/sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			Code:        http.StatusOK,
			Header:      map[string]string{"Docker-Content-Digest": "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
			Want:        "foo",
		},
		{
			Description: "HEAD blob",
			Digests:     map[string]string{"sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae": "foo"},
			Method:      "HEAD",
			URL:         "/v2/foo/blobs/sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			Code:        http.StatusOK,
			Header: map[string]string{
				"Content-Length":        "3",
				"Docker-Content-Digest": "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			},
		},
		{
			Description: "DELETE blob",
			Digests:     map[string]string{"sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae": "foo"},
			Method:      "DELETE",
			URL:         "/v2/foo/blobs/sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			Code:        http.StatusAccepted,
		},
		{
			Description: "blob url with no container",
			Method:      "GET",
			URL:         "/v2/blobs/sha256:asd",
			Code:        http.StatusBadRequest,
		},
		{
			Description: "uploadurl",
			Method:      "POST",
			URL:         "/v2/foo/blobs/uploads",
			Code:        http.StatusAccepted,
			Header:      map[string]string{"Range": "0-0"},
		},
		{
			Description: "uploadurl",
			Method:      "POST",
			URL:         "/v2/foo/blobs/uploads/",
			Code:        http.StatusAccepted,
			Header:      map[string]string{"Range": "0-0"},
		},
		{
			Description: "upload put missing digest",
			Method:      "PUT",
			URL:         "/v2/foo/blobs/uploads/1",
			Code:        http.StatusBadRequest,
		},
		{
			Description: "monolithic upload good digest",
			Method:      "POST",
			URL:         "/v2/foo/blobs/uploads?digest=sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			Code:        http.StatusCreated,
			Body:        "foo",
			Header:      map[string]string{"Docker-Content-Digest": "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
		},
		{
			Description: "monolithic upload bad digest",
			Method:      "POST",
			URL:         "/v2/foo/blobs/uploads?digest=sha256:fake",
			Code:        http.StatusBadRequest,
			Body:        "foo",
		},
		{
			Description: "upload good digest",
			Method:      "PUT",
			URL:         "/v2/foo/blobs/uploads/1?digest=sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			Code:        http.StatusCreated,
			Body:        "foo",
			Header:      map[string]string{"Docker-Content-Digest": "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
		},
		{
			Description: "upload bad digest",
			Method:      "PUT",
			URL:         "/v2/foo/blobs/uploads/1?digest=sha256:baddigest",
			Code:        http.StatusBadRequest,
			Body:        "foo",
		},
		{
			Description: "stream upload",
			Method:      "PATCH",
			URL:         "/v2/foo/blobs/uploads/1",
			Code:        http.StatusNoContent,
			Body:        "foo",
			Header: map[string]string{
				"Range":    "0-2",
				"Location": "/v2/foo/blobs/uploads/1",
			},
		},
		{
			Description: "stream duplicate upload",
			Method:      "PATCH",
			URL:         "/v2/foo/blobs/uploads/1",
			Code:        http.StatusBadRequest,
			Body:        "foo",
			BlobStream:  map[string]string{"1": "foo"},
		},
		{
			Description: "stream finish upload",
			Method:      "PUT",
			URL:         "/v2/foo/blobs/uploads/1?digest=sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			BlobStream:  map[string]string{"1": "foo"},
			Code:        http.StatusCreated,
			Header:      map[string]string{"Docker-Content-Digest": "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"},
		},
		{
			Description: "get missing manifest",
			Method:      "GET",
			URL:         "/v2/foo/manifests/latest",
			Code:        http.StatusNotFound,
		},
		{
			Description: "head missing manifest",
			Method:      "HEAD",
			URL:         "/v2/foo/manifests/latest",
			Code:        http.StatusNotFound,
		},
		{
			Description: "get missing manifest good container",
			Manifests:   map[string]string{"foo/manifests/latest": "foo"},
			Method:      "GET",
			URL:         "/v2/foo/manifests/bar",
			Code:        http.StatusNotFound,
		},
		{
			Description: "head missing manifest good container",
			Manifests:   map[string]string{"foo/manifests/latest": "foo"},
			Method:      "HEAD",
			URL:         "/v2/foo/manifests/bar",
			Code:        http.StatusNotFound,
		},
		{
			Description: "get manifest by tag",
			Manifests:   map[string]string{"foo/manifests/latest": "foo"},
			Method:      "GET",
			URL:         "/v2/foo/manifests/latest",
			Code:        http.StatusOK,
			Want:        "foo",
		},
		{
			Description: "get manifest by digest",
			Manifests:   map[string]string{"foo/manifests/latest": "foo"},
			Method:      "GET",
			URL:         "/v2/foo/manifests/sha256:" + sha256String("foo"),
			Code:        http.StatusOK,
			Want:        "foo",
		},
		{
			Description: "head manifest",
			Manifests:   map[string]string{"foo/manifests/latest": "foo"},
			Method:      "HEAD",
			URL:         "/v2/foo/manifests/latest",
			Code:        http.StatusOK,
		},
		{
			Description: "create manifest",
			Method:      "PUT",
			URL:         "/v2/foo/manifests/latest",
			Code:        http.StatusCreated,
			Body:        "foo",
		},
		{
			Description: "create index",
			Method:      "PUT",
			URL:         "/v2/foo/manifests/latest",
			Code:        http.StatusCreated,
			Body:        weirdIndex,
			RequestHeader: map[string]string{
				"Content-Type": "application/vnd.oci.image.index.v1+json",
			},
			Manifests: map[string]string{"foo/manifests/image": "foo"},
		},
		{
			Description: "create index missing child",
			Method:      "PUT",
			URL:         "/v2/foo/manifests/latest",
			Code:        http.StatusNotFound,
			Body:        weirdIndex,
			RequestHeader: map[string]string{
				"Content-Type": "application/vnd.oci.image.index.v1+json",
			},
		},
		{
			Description: "bad index body",
			Method:      "PUT",
			URL:         "/v2/foo/manifests/latest",
			Code:        http.StatusBadRequest,
			Body:        "foo",
			RequestHeader: map[string]string{
				"Content-Type": "application/vnd.oci.image.index.v1+json",
			},
		},
		{
			Description: "bad manifest method",
			Method:      "BAR",
			URL:         "/v2/foo/manifests/latest",
			Code:        http.StatusBadRequest,
		},
		{
			Description:   "Chunk upload start",
			Method:        "PATCH",
			URL:           "/v2/foo/blobs/uploads/1",
			RequestHeader: map[string]string{"Content-Range": "0-3"},
			Code:          http.StatusNoContent,
			Body:          "foo",
			Header: map[string]string{
				"Range":    "0-2",
				"Location": "/v2/foo/blobs/uploads/1",
			},
		},
		{
			Description:   "Chunk upload bad content range",
			Method:        "PATCH",
			URL:           "/v2/foo/blobs/uploads/1",
			RequestHeader: map[string]string{"Content-Range": "0-bar"},
			Code:          http.StatusRequestedRangeNotSatisfiable,
			Body:          "foo",
		},
		{
			Description:   "Chunk upload overlaps previous data",
			Method:        "PATCH",
			URL:           "/v2/foo/blobs/uploads/1",
			BlobStream:    map[string]string{"1": "foo"},
			RequestHeader: map[string]string{"Content-Range": "2-5"},
			Code:          http.StatusRequestedRangeNotSatisfiable,
			Body:          "bar",
		},
		{
			Description:   "Chunk upload after previous data",
			Method:        "PATCH",
			URL:           "/v2/foo/blobs/uploads/1",
			BlobStream:    map[string]string{"1": "foo"},
			RequestHeader: map[string]string{"Content-Range": "3-6"},
			Code:          http.StatusNoContent,
			Body:          "bar",
			Header: map[string]string{
				"Range":    "0-5",
				"Location": "/v2/foo/blobs/uploads/1",
			},
		},
		{
			Description: "DELETE Unknown name",
			Method:      "DELETE",
			URL:         "/v2/test/honk/manifests/latest",
			Code:        http.StatusNotFound,
		},
		{
			Description: "DELETE Unknown manifest",
			Manifests:   map[string]string{"honk/manifests/latest": "honk"},
			Method:      "DELETE",
			URL:         "/v2/honk/manifests/tag-honk",
			Code:        http.StatusNotFound,
		},
		{
			Description: "DELETE existing manifest",
			Manifests:   map[string]string{"foo/manifests/latest": "foo"},
			Method:      "DELETE",
			URL:         "/v2/foo/manifests/latest",
			Code:        http.StatusAccepted,
		},
		{
			Description: "DELETE existing manifest by digest",
			Manifests:   map[string]string{"foo/manifests/latest": "foo"},
			Method:      "DELETE",
			URL:         "/v2/foo/manifests/sha256:" + sha256String("foo"),
			Code:        http.StatusAccepted,
		},
		{
			Description: "list tags",
			Manifests:   map[string]string{"foo/manifests/latest": "foo", "foo/manifests/tag1": "foo"},
			Method:      "GET",
			URL:         "/v2/foo/tags/list?n=1000",
			Code:        http.StatusOK,
			Want:        `{"name":"foo","tags":["latest","tag1"]}`,
		},
		{
			Description: "limit tags",
			Manifests:   map[string]string{"foo/manifests/latest": "foo", "foo/manifests/tag1": "foo"},
			Method:      "GET",
			URL:         "/v2/foo/tags/list?n=1",
			Code:        http.StatusOK,
			Want:        `{"name":"foo","tags":["latest"]}`,
		},
		{
			Description: "offset tags",
			Manifests:   map[string]string{"foo/manifests/latest": "foo", "foo/manifests/tag1": "foo"},
			Method:      "GET",
			URL:         "/v2/foo/tags/list?last=latest",
			Code:        http.StatusOK,
			Want:        `{"name":"foo","tags":["tag1"]}`,
		},
		{
			Description: "list non existing tags",
			Method:      "GET",
			URL:         "/v2/foo/tags/list?n=1000",
			Code:        http.StatusNotFound,
		},
		{
			Description: "list repos",
			Manifests:   map[string]string{"foo/manifests/latest": "foo", "bar/manifests/latest": "bar"},
			Method:      "GET",
			URL:         "/v2/_catalog?n=1000",
			Code:        http.StatusOK,
		},
		{
			Description: "fetch references",
			Method:      "GET",
			URL:         "/v2/foo/referrers/sha256:" + sha256String("foo"),
			Code:        http.StatusOK,
			Manifests: map[string]string{
				"foo/manifests/image":           "foo",
				"foo/manifests/points-to-image": "{\"subject\": {\"digest\": \"sha256:" + sha256String("foo") + "\"}}",
			},
		},
		{
			Description: "fetch references, subject pointing elsewhere",
			Method:      "GET",
			URL:         "/v2/foo/referrers/sha256:" + sha256String("foo"),
			Code:        http.StatusOK,
			Manifests: map[string]string{
				"foo/manifests/image":           "foo",
				"foo/manifests/points-to-image": "{\"subject\": {\"digest\": \"sha256:" + sha256String("nonexistant") + "\"}}",
			},
		},
		{
			Description: "fetch references, no results",
			Method:      "GET",
			URL:         "/v2/foo/referrers/sha256:" + sha256String("foo"),
			Code:        http.StatusOK,
			Manifests: map[string]string{
				"foo/manifests/image": "foo",
			},
		},
		{
			Description: "fetch references, missing repo",
			Method:      "GET",
			URL:         "/v2/does-not-exist/referrers/sha256:" + sha256String("foo"),
			Code:        http.StatusNotFound,
		},
		{
			Description: "fetch references, bad target (tag vs. digest)",
			Method:      "GET",
			URL:         "/v2/foo/referrers/latest",
			Code:        http.StatusBadRequest,
		},
		{
			Description: "fetch references, bad method",
			Method:      "POST",
			URL:         "/v2/foo/referrers/sha256:" + sha256String("foo"),
			Code:        http.StatusBadRequest,
		},
	}

	for _, tc := range tcs {

		var logger *log.Logger
		testf := func(t *testing.T) {

			opts := []registry.Option{registry.WithReferrersSupport(true)}
			if logger != nil {
				opts = append(opts, registry.Logger(logger))
			}
			r := registry.New(opts...)
			s := httptest.NewServer(r)
			defer s.Close()

			for manifest, contents := range tc.Manifests {
				u, err := url.Parse(s.URL + "/v2/" + manifest)
				if err != nil {
					t.Fatalf("Error parsing %q: %v", s.URL+"/v2", err)
				}
				req := &http.Request{
					Method: "PUT",
					URL:    u,
					Body:   io.NopCloser(strings.NewReader(contents)),
				}
				t.Log(req.Method, req.URL)
				resp, err := s.Client().Do(req)
				if err != nil {
					t.Fatalf("Error uploading manifest: %v", err)
				}
				if resp.StatusCode != http.StatusCreated {
					body, _ := io.ReadAll(resp.Body)
					t.Fatalf("Error uploading manifest got status: %d %s", resp.StatusCode, body)
				}
				t.Logf("created manifest with digest %v", resp.Header.Get("Docker-Content-Digest"))
			}

			for digest, contents := range tc.Digests {
				u, err := url.Parse(fmt.Sprintf("%s/v2/foo/blobs/uploads/1?digest=%s", s.URL, digest))
				if err != nil {
					t.Fatalf("Error parsing %q: %v", s.URL+tc.URL, err)
				}
				req := &http.Request{
					Method: "PUT",
					URL:    u,
					Body:   io.NopCloser(strings.NewReader(contents)),
				}
				t.Log(req.Method, req.URL)
				resp, err := s.Client().Do(req)
				if err != nil {
					t.Fatalf("Error uploading digest: %v", err)
				}
				if resp.StatusCode != http.StatusCreated {
					body, _ := io.ReadAll(resp.Body)
					t.Fatalf("Error uploading digest got status: %d %s", resp.StatusCode, body)
				}
			}

			for upload, contents := range tc.BlobStream {
				u, err := url.Parse(fmt.Sprintf("%s/v2/foo/blobs/uploads/%s", s.URL, upload))
				if err != nil {
					t.Fatalf("Error parsing %q: %v", s.URL+tc.URL, err)
				}
				req := &http.Request{
					Method: "PATCH",
					URL:    u,
					Body:   io.NopCloser(strings.NewReader(contents)),
				}
				t.Log(req.Method, req.URL)
				resp, err := s.Client().Do(req)
				if err != nil {
					t.Fatalf("Error streaming blob: %v", err)
				}
				if resp.StatusCode != http.StatusNoContent {
					body, _ := io.ReadAll(resp.Body)
					t.Fatalf("Error streaming blob: %d %s", resp.StatusCode, body)
				}

			}

			u, err := url.Parse(s.URL + tc.URL)
			if err != nil {
				t.Fatalf("Error parsing %q: %v", s.URL+tc.URL, err)
			}
			req := &http.Request{
				Method: tc.Method,
				URL:    u,
				Body:   io.NopCloser(strings.NewReader(tc.Body)),
				Header: map[string][]string{},
			}
			for k, v := range tc.RequestHeader {
				req.Header.Set(k, v)
			}
			t.Log(req.Method, req.URL)
			resp, err := s.Client().Do(req)
			if err != nil {
				t.Fatalf("Error getting %q: %v", tc.URL, err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("Reading response body: %v", err)
			}
			if resp.StatusCode != tc.Code {
				t.Errorf("Incorrect status code, got %d, want %d; body: %s", resp.StatusCode, tc.Code, body)
			}

			for k, v := range tc.Header {
				r := resp.Header.Get(k)
				if r != v {
					t.Errorf("Incorrect header %q received, got %q, want %q", k, r, v)
				}
			}

			if tc.Want != "" && string(body) != tc.Want {
				t.Errorf("Incorrect response body, got %q, want %q", body, tc.Want)
			}
		}
		t.Run(tc.Description, testf)
		logger = log.New(io.Discard, "", log.Ldate)
		t.Run(tc.Description+" - custom log", testf)
	}
}
