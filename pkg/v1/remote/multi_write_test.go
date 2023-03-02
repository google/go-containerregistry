// Copyright 2020 Google LLC All Rights Reserved.
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
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestMultiWrite(t *testing.T) {
	// Create a random image.
	img1, err := random.Image(1024, 2)
	if err != nil {
		t.Fatal("random.Image:", err)
	}

	// Create another image that's based on the first.
	rl, err := random.Layer(1024, types.OCIUncompressedLayer)
	if err != nil {
		t.Fatal("random.Layer:", err)
	}
	img2, err := mutate.AppendLayers(img1, rl)
	if err != nil {
		t.Fatal("mutate.AppendLayers:", err)
	}

	// Also create a random index of images.
	subidx, err := random.Index(1024, 2, 3)
	if err != nil {
		t.Fatal("random.Index:", err)
	}

	// Add a sub-sub-index of random images.
	subsubidx, err := random.Index(1024, 3, 4)
	if err != nil {
		t.Fatal("random.Index:", err)
	}
	subidx = mutate.AppendManifests(subidx, mutate.IndexAddendum{Add: subsubidx})

	// Create an index containing both images and the index above.
	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: img1},
		mutate.IndexAddendum{Add: img2},
		mutate.IndexAddendum{Add: subidx},
		mutate.IndexAddendum{Add: rl},
	)

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Write both images and the manifest list.
	tag1, tag2, tag3 := mustNewTag(t, u.Host+"/repo:tag1"), mustNewTag(t, u.Host+"/repo:tag2"), mustNewTag(t, u.Host+"/repo:tag3")
	if err := MultiWrite(map[name.Reference]Taggable{
		tag1: img1,
		tag2: img2,
		tag3: idx,
	}); err != nil {
		t.Error("Write:", err)
	}

	// Check that tagged images are present.
	for _, tag := range []name.Tag{tag1, tag2} {
		got, err := Image(tag)
		if err != nil {
			t.Error(err)
			continue
		}
		if err := validate.Image(got); err != nil {
			t.Error("Validate() =", err)
		}
	}

	// Check that tagged manfest list is present and valid.
	got, err := Index(tag3)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Index(got); err != nil {
		t.Error("Validate() =", err)
	}
}

func TestMultiWriteWithNondistributableLayer(t *testing.T) {
	// Create a random image.
	img1, err := random.Image(1024, 2)
	if err != nil {
		t.Fatal("random.Image:", err)
	}

	// Create another image that's based on the first.
	rl, err := random.Layer(1024, types.OCIRestrictedLayer)
	if err != nil {
		t.Fatal("random.Layer:", err)
	}
	img, err := mutate.AppendLayers(img1, rl)
	if err != nil {
		t.Fatal("mutate.AppendLayers:", err)
	}

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Write the image.
	tag1 := mustNewTag(t, u.Host+"/repo:tag1")
	if err := MultiWrite(map[name.Reference]Taggable{tag1: img}, WithNondistributable); err != nil {
		t.Error("Write:", err)
	}

	// Check that tagged image is present.
	got, err := Image(tag1)
	if err != nil {
		t.Error(err)
	}
	if err := validate.Image(got); err != nil {
		t.Error("Validate() =", err)
	}
}

func TestMultiWrite_Retry(t *testing.T) {
	// Create a random image.
	img1, err := random.Image(1024, 2)
	if err != nil {
		t.Fatal("random.Image:", err)
	}

	t.Run("retry http error 500", func(t *testing.T) {
		// Set up a fake registry.
		handler := registry.New()

		numOfInternalServerErrors := 0
		registryThatFailsOnFirstUpload := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if strings.Contains(request.URL.Path, "/manifests/") && numOfInternalServerErrors < 1 {
				numOfInternalServerErrors++
				responseWriter.WriteHeader(500)
				return
			}
			handler.ServeHTTP(responseWriter, request)
		})

		s := httptest.NewServer(registryThatFailsOnFirstUpload)
		defer s.Close()
		u, err := url.Parse(s.URL)
		if err != nil {
			t.Fatal(err)
		}

		tag1 := mustNewTag(t, u.Host+"/repo:tag1")
		if err := MultiWrite(map[name.Reference]Taggable{
			tag1: img1,
		}, WithRetryBackoff(fastBackoff)); err != nil {
			t.Error("Write:", err)
		}
	})

	t.Run("do not retry http error 401", func(t *testing.T) {
		// Set up a fake registry.
		handler := registry.New()

		numOf401HttpErrors := 0
		registryThatFailsOnFirstUpload := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if strings.Contains(request.URL.Path, "/manifests/") {
				numOf401HttpErrors++
				responseWriter.WriteHeader(401)
				return
			}
			handler.ServeHTTP(responseWriter, request)
		})

		s := httptest.NewServer(registryThatFailsOnFirstUpload)
		defer s.Close()
		u, err := url.Parse(s.URL)
		if err != nil {
			t.Fatal(err)
		}

		tag1 := mustNewTag(t, u.Host+"/repo:tag1")
		if err := MultiWrite(map[name.Reference]Taggable{
			tag1: img1,
		}); err == nil {
			t.Fatal("Expected error:")
		}

		if numOf401HttpErrors > 1 {
			t.Fatal("Should not retry on 401 errors:")
		}
	})

	t.Run("do not retry transport errors if transport.Wrapper is used", func(t *testing.T) {
		// reference a http server that is not listening (used to pick a port that isn't listening)
		onlyHandlesPing := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if strings.HasSuffix(request.URL.Path, "/v2/") {
				responseWriter.WriteHeader(200)
				return
			}
		})
		s := httptest.NewServer(onlyHandlesPing)
		defer s.Close()

		u, err := url.Parse(s.URL)
		if err != nil {
			t.Fatal(err)
		}

		tag1 := mustNewTag(t, u.Host+"/repo:tag1")

		// using a transport.Wrapper, meaning retry logic should not be wrapped
		doesNotRetryTransport := &countTransport{inner: http.DefaultTransport}
		transportWrapper, err := transport.NewWithContext(context.Background(), tag1.Repository.Registry, authn.Anonymous, doesNotRetryTransport, nil)
		if err != nil {
			t.Fatal(err)
		}

		noRetry := func(error) bool { return false }

		if err := MultiWrite(map[name.Reference]Taggable{
			tag1: img1,
		}, WithTransport(transportWrapper), WithJobs(1), WithRetryPredicate(noRetry)); err == nil {
			t.Errorf("Expected an error, got nil")
		}

		// expect count == 1 since jobs is set to 1 and we should not retry on transport eof error
		if doesNotRetryTransport.count != 1 {
			t.Errorf("Incorrect count, got %d, want %d", doesNotRetryTransport.count, 1)
		}
	})

	t.Run("do not add UserAgent if transport.Wrapper is used", func(t *testing.T) {
		expectedNotUsedUserAgent := "TEST_USER_AGENT"

		handler := registry.New()

		registryThatAssertsUserAgentIsCorrect := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if strings.Contains(request.Header.Get("User-Agent"), expectedNotUsedUserAgent) {
				t.Fatalf("Should not contain User-Agent: %s, Got: %s", expectedNotUsedUserAgent, request.Header.Get("User-Agent"))
			}

			handler.ServeHTTP(responseWriter, request)
		})

		s := httptest.NewServer(registryThatAssertsUserAgentIsCorrect)

		defer s.Close()
		u, err := url.Parse(s.URL)
		if err != nil {
			t.Fatal(err)
		}

		tag1 := mustNewTag(t, u.Host+"/repo:tag1")
		// using a transport.Wrapper, meaning retry logic should not be wrapped
		transportWrapper, err := transport.NewWithContext(context.Background(), tag1.Repository.Registry, authn.Anonymous, http.DefaultTransport, nil)
		if err != nil {
			t.Fatal(err)
		}

		if err := MultiWrite(map[name.Reference]Taggable{
			tag1: img1,
		}, WithTransport(transportWrapper), WithUserAgent(expectedNotUsedUserAgent)); err != nil {
			t.Fatal(err)
		}
	})
}

// TestMultiWrite_Deep tests that a deeply nested tree of manifest lists gets
// pushed in the correct order (i.e., each level in sequence).
func TestMultiWrite_Deep(t *testing.T) {
	idx, err := random.Index(1024, 2, 2)
	if err != nil {
		t.Fatal("random.Image:", err)
	}
	for i := 0; i < 4; i++ {
		idx = mutate.AppendManifests(idx, mutate.IndexAddendum{Add: idx})
	}

	// Set up a fake registry (with NOP logger to avoid spamming test logs).
	nopLog := log.New(io.Discard, "", 0)
	s := httptest.NewServer(registry.New(registry.Logger(nopLog)))
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Write both images and the manifest list.
	tag := mustNewTag(t, u.Host+"/repo:tag")
	if err := MultiWrite(map[name.Reference]Taggable{
		tag: idx,
	}); err != nil {
		t.Error("Write:", err)
	}

	// Check that tagged manfest list is present and valid.
	got, err := Index(tag)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Index(got); err != nil {
		t.Error("Validate() =", err)
	}
}

type countTransport struct {
	count int
	inner http.RoundTripper
}

func (t *countTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Path, "/v2/") {
		return t.inner.RoundTrip(req)
	}

	t.count++
	return nil, io.ErrUnexpectedEOF
}
