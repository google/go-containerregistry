package registry_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
)

func TestCalls(t *testing.T) {
	tcs := []struct {
		Description string

		// Request / setup
		URL     string
		Digests map[string]string

		// Response
		Code   int
		Header map[string]string
		Method string
		Body   string
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
			URL:         "/v2/foo/blobs/sha256:asd",
			Code:        http.StatusNotFound,
		},
		{
			Description: "HEAD non existent blob",
			Method:      "HEAD",
			URL:         "/v2/foo/blobs/sha256:asd",
			Code:        http.StatusNotFound,
		},
		{
			Description: "GET blob",
			Digests:     map[string]string{"sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae": "foo"},
			Method:      "GET",
			URL:         "/v2/foo/blobs/sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			Code:        http.StatusOK,
		},
		{
			Description: "HEAD blob",
			Digests:     map[string]string{"sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae": "foo"},
			Method:      "HEAD",
			URL:         "/v2/foo/blobs/sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			Code:        http.StatusOK,
			Header:      map[string]string{"Content-Length": "3"},
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
			Description: "upload put missing digest",
			Method:      "PUT",
			URL:         "/v2/foo/blobs/uploads/1",
			Code:        http.StatusBadRequest,
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
	}

	for _, tc := range tcs {
		t.Run(tc.Description, func(t *testing.T) {
			s := httptest.NewServer(registry.New())
			defer s.Close()

			for digest, contents := range tc.Digests {
				u, err := url.Parse(fmt.Sprintf("%s/v2/foo/blobs/uploads/1?digest=%s", s.URL, digest))
				if err != nil {
					t.Fatalf("Error parsing %q: %v", s.URL+tc.URL, err)
				}
				req := &http.Request{
					Method: "PUT",
					URL:    u,
					Body:   ioutil.NopCloser(strings.NewReader(contents)),
				}
				resp, err := s.Client().Do(req)
				if err != nil {
					t.Fatalf("Error uploading digest: %v", err)
				}
				if resp.StatusCode != http.StatusCreated {
					t.Fatalf("Error uploading digest got status: %d", resp.StatusCode)
				}

			}

			u, err := url.Parse(s.URL + tc.URL)
			if err != nil {
				t.Fatalf("Error parsing %q: %v", s.URL+tc.URL, err)
			}
			req := &http.Request{
				Method: tc.Method,
				URL:    u,
				Body:   ioutil.NopCloser(strings.NewReader(tc.Body)),
			}
			resp, err := s.Client().Do(req)
			if err != nil {
				t.Fatalf("Error getting %q: %v", tc.URL, err)
			}
			if resp.StatusCode != tc.Code {
				t.Errorf("Incorrect status code, got %d, want %d", resp.StatusCode, tc.Code)
			}

			for k, v := range tc.Header {
				r := resp.Header.Get(k)
				if r != v {
					t.Errorf("Incorrect header %q received, got %q, want %q", k, r, v)
				}
			}
		})
	}
}
