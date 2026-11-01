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

package remote_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestReferrers(t *testing.T) {
	// Run all tests against:
	//
	//   (1) A OCI 1.0 registry (without referrers API)
	//   (2) An OCI 1.1+ registry (with referrers API)
	//
	for _, leg := range []struct {
		server      *httptest.Server
		tryFallback bool
	}{
		{
			server:      httptest.NewServer(registry.New(registry.WithReferrersSupport(false))),
			tryFallback: true,
		},
		{
			server:      httptest.NewServer(registry.New(registry.WithReferrersSupport(true))),
			tryFallback: false,
		},
	} {
		s := leg.server
		defer s.Close()
		u, err := url.Parse(s.URL)
		if err != nil {
			t.Fatal(err)
		}

		annotations := map[string]string{
			"org.example.testing123": "testing456",
		}
		descriptor := func(img v1.Image) v1.Descriptor {
			d, err := img.Digest()
			if err != nil {
				t.Fatal(err)
			}
			sz, err := img.Size()
			if err != nil {
				t.Fatal(err)
			}
			mt, err := img.MediaType()
			if err != nil {
				t.Fatal(err)
			}
			return v1.Descriptor{
				Digest:       d,
				Size:         sz,
				MediaType:    mt,
				ArtifactType: "application/testing123",
				Annotations:  annotations,
			}
		}

		// Push an image we'll attach things to.
		// We'll copy from src to dst.
		rootRef, err := name.ParseReference(fmt.Sprintf("%s/repo:root", u.Host))
		if err != nil {
			t.Fatal(err)
		}
		rootImg, err := random.Image(10, 10)
		if err != nil {
			t.Fatal(err)
		}
		rootImg = mutate.ConfigMediaType(rootImg, types.MediaType("application/testing123"))
		rootImg = mutate.Annotations(rootImg, annotations).(v1.Image)
		if err := remote.Write(rootRef, rootImg); err != nil {
			t.Fatal(err)
		}
		rootDesc := descriptor(rootImg)
		t.Logf("root image is %s", rootDesc.Digest)

		// Before pushing referrers, try to get the referrers of the root image.
		rootRefDigest := rootRef.Context().Digest(rootDesc.Digest.String())
		index, err := remote.Referrers(rootRefDigest)
		if err != nil {
			t.Fatal(err)
		}
		m, err := index.IndexManifest()
		if err != nil {
			t.Fatal(err)
		}
		if numManifests := len(m.Manifests); numManifests != 0 {
			t.Fatalf("expected index to contain 0 manifests, but had %d", numManifests)
		}

		// Push an image that refers to the root image as its subject.
		leafRef, err := name.ParseReference(fmt.Sprintf("%s/repo:leaf", u.Host))
		if err != nil {
			t.Fatal(err)
		}
		leafImg, err := random.Image(20, 20)
		if err != nil {
			t.Fatal(err)
		}
		leafImg = mutate.ConfigMediaType(leafImg, types.MediaType("application/testing123"))
		leafImg = mutate.Annotations(leafImg, annotations).(v1.Image)
		leafImg = mutate.Subject(leafImg, rootDesc).(v1.Image)
		if err := remote.Write(leafRef, leafImg); err != nil {
			t.Fatal(err)
		}
		leafDesc := descriptor(leafImg)
		t.Logf("leaf image is %s", leafDesc.Digest)

		// Get the referrers of the root image, by digest.
		index, err = remote.Referrers(rootRefDigest)
		if err != nil {
			t.Fatal(err)
		}
		m2, err := index.IndexManifest()
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff([]v1.Descriptor{leafDesc}, m2.Manifests); d != "" {
			t.Logf("m2.Manifests: %v", m2.Manifests)
			t.Fatalf("referrers diff (-want,+got): %s", d)
		}

		if leg.tryFallback {
			// Get the referrers by querying the root image's fallback tag directly.
			tag, err := name.ParseReference(fmt.Sprintf("%s/repo:sha256-%s", u.Host, rootDesc.Digest.Hex))
			if err != nil {
				t.Fatal(err)
			}
			idx, err := remote.Index(tag)
			if err != nil {
				t.Fatal(err)
			}
			mf, err := idx.IndexManifest()
			if err != nil {
				t.Fatal(err)
			}
			m2, err := index.IndexManifest()
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(m2.Manifests, mf.Manifests); d != "" {
				t.Fatalf("fallback tag diff (-want,+got): %s", d)
			}
		}

		// Push the leaf image again, this time with a different tag.
		// This shouldn't add another item to the root image's referrers,
		// because it's the same digest.
		// Push an image that refers to the root image as its subject.
		leaf2Ref, err := name.ParseReference(fmt.Sprintf("%s/repo:leaf2", u.Host))
		if err != nil {
			t.Fatal(err)
		}
		if err := remote.Write(leaf2Ref, leafImg); err != nil {
			t.Fatal(err)
		}
		// Get the referrers of the root image again, which should only have one entry.
		rootRefDigest = rootRef.Context().Digest(rootDesc.Digest.String())
		index, err = remote.Referrers(rootRefDigest)
		if err != nil {
			t.Fatal(err)
		}
		m3, err := index.IndexManifest()
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff([]v1.Descriptor{leafDesc}, m3.Manifests); d != "" {
			t.Fatalf("referrers diff after second push (-want,+got): %s", d)
		}

		// Try applying filters and verify number of manifests and and annotations
		index, err = remote.Referrers(rootRefDigest,
			remote.WithFilter("artifactType", "application/testing123"))
		if err != nil {
			t.Fatal(err)
		}
		m4, err := index.IndexManifest()
		if err != nil {
			t.Fatal(err)
		}
		if numManifests := len(m4.Manifests); numManifests == 0 {
			t.Fatal("index contained 0 manifests")
		}

		index, err = remote.Referrers(rootRefDigest,
			remote.WithFilter("artifactType", "application/testing123BADDDD"))
		if err != nil {
			t.Fatal(err)
		}
		m5, err := index.IndexManifest()
		if err != nil {
			t.Fatal(err)
		}
		if numManifests := len(m5.Manifests); numManifests != 0 {
			t.Fatalf("expected index to contain 0 manifests, but had %d", numManifests)
		}
	}
}

func TestReferrersTagFallbackDisabled(t *testing.T) {
	// Push a subject image and an image referring to it, with the fallback
	// tag scheme disabled.
	pushWithSubject := func(t *testing.T, host string) (rootDigest v1.Hash, leafDigest v1.Hash, err error) {
		rootRef, err := name.ParseReference(fmt.Sprintf("%s/repo:root", host))
		if err != nil {
			t.Fatal(err)
		}
		rootImg, err := random.Image(10, 10)
		if err != nil {
			t.Fatal(err)
		}
		if err := remote.Write(rootRef, rootImg, remote.WithReferrersTagFallback(false)); err != nil {
			t.Fatal(err)
		}
		rootDigest, err = rootImg.Digest()
		if err != nil {
			t.Fatal(err)
		}
		rootSize, err := rootImg.Size()
		if err != nil {
			t.Fatal(err)
		}
		rootMediaType, err := rootImg.MediaType()
		if err != nil {
			t.Fatal(err)
		}

		leafRef, err := name.ParseReference(fmt.Sprintf("%s/repo:leaf", host))
		if err != nil {
			t.Fatal(err)
		}
		leafImg, err := random.Image(20, 20)
		if err != nil {
			t.Fatal(err)
		}
		leafImg = mutate.Subject(leafImg, v1.Descriptor{
			Digest:    rootDigest,
			Size:      rootSize,
			MediaType: rootMediaType,
		}).(v1.Image)
		leafDigest, err = leafImg.Digest()
		if err != nil {
			t.Fatal(err)
		}
		return rootDigest, leafDigest, remote.Write(leafRef, leafImg, remote.WithReferrersTagFallback(false))
	}

	// The fallback tag should never be created with the fallback disabled.
	checkNoFallbackTag := func(t *testing.T, host string, rootDigest v1.Hash) {
		fallbackRef, err := name.ParseReference(fmt.Sprintf("%s/repo:sha256-%s", host, rootDigest.Hex))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := remote.Head(fallbackRef); err == nil {
			t.Errorf("Head(%q) succeeded, want NotFound", fallbackRef)
		} else {
			var terr *transport.Error
			if !errors.As(err, &terr) || terr.StatusCode != http.StatusNotFound {
				t.Errorf("Head(%q) = %v, want NotFound", fallbackRef, err)
			}
		}
	}

	referrersDigest := func(t *testing.T, host string, rootDigest v1.Hash) name.Digest {
		repo, err := name.NewRepository(fmt.Sprintf("%s/repo", host))
		if err != nil {
			t.Fatal(err)
		}
		return repo.Digest(rootDigest.String())
	}

	t.Run("registry without referrers API", func(t *testing.T) {
		// An OCI 1.0 registry (without referrers API), instrumented to record
		// requests to the referrers endpoint.
		reg := registry.New()
		var mu sync.Mutex
		referrersProbes := 0
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/referrers/") {
				mu.Lock()
				referrersProbes++
				mu.Unlock()
			}
			reg.ServeHTTP(w, r)
		}))
		defer s.Close()
		u, err := url.Parse(s.URL)
		if err != nil {
			t.Fatal(err)
		}

		// Pushing a subject-bearing manifest requires the Referrers API.
		rootDigest, _, err := pushWithSubject(t, u.Host)
		if err == nil {
			t.Fatal("Write() succeeded, want error")
		} else if !strings.Contains(err.Error(), "referrers tag fallback is disabled") {
			t.Fatalf("Write() = %v, want referrers tag fallback error", err)
		}

		mu.Lock()
		probes := referrersProbes
		mu.Unlock()
		if probes != 1 {
			t.Errorf("pushing probed the referrers endpoint %d times, want 1", probes)
		}

		checkNoFallbackTag(t, u.Host, rootDigest)

		// Listing referrers requires the Referrers API too.
		if _, err := remote.Referrers(referrersDigest(t, u.Host, rootDigest), remote.WithReferrersTagFallback(false)); err == nil {
			t.Fatal("Referrers() succeeded, want error")
		} else if !strings.Contains(err.Error(), "referrers tag fallback is disabled") {
			t.Fatalf("Referrers() = %v, want referrers tag fallback error", err)
		}
	})

	t.Run("registry with referrers API", func(t *testing.T) {
		s := httptest.NewServer(registry.New(registry.WithReferrersSupport(true)))
		defer s.Close()
		u, err := url.Parse(s.URL)
		if err != nil {
			t.Fatal(err)
		}

		rootDigest, leafDigest, err := pushWithSubject(t, u.Host)
		if err != nil {
			t.Fatal(err)
		}

		checkNoFallbackTag(t, u.Host, rootDigest)

		// The registry serves the referrer natively.
		index, err := remote.Referrers(referrersDigest(t, u.Host, rootDigest), remote.WithReferrersTagFallback(false))
		if err != nil {
			t.Fatal(err)
		}
		m, err := index.IndexManifest()
		if err != nil {
			t.Fatal(err)
		}
		if len(m.Manifests) != 1 || m.Manifests[0].Digest != leafDigest {
			t.Fatalf("referrers = %v, want one entry for %s", m.Manifests, leafDigest)
		}
	})
}

func TestReferrersPagination(t *testing.T) {
	subject := "sha256:" + strings.Repeat("a", 64)
	path := "/v2/repo/referrers/" + subject

	// Five referrers to spread across pages.
	referrers := make([]v1.Descriptor, 5)
	for i := range referrers {
		referrers[i] = v1.Descriptor{
			MediaType:    types.OCIManifestSchema1,
			Digest:       v1.Hash{Algorithm: "sha256", Hex: fmt.Sprintf("%064d", i)},
			Size:         int64(i + 1),
			ArtifactType: fmt.Sprintf("application/testing%d", i),
		}
	}
	writeIndex := func(t *testing.T, w http.ResponseWriter, manifests []v1.Descriptor) {
		if err := json.NewEncoder(w).Encode(&v1.IndexManifest{
			SchemaVersion: 2,
			MediaType:     types.OCIImageIndex,
			Manifests:     manifests,
		}); err != nil {
			t.Error(err)
		}
	}

	// A page of the paginated referrers response, served at path?page=<i>.
	type page struct {
		status      int    // defaults to 200
		contentType string // defaults to the OCI image index media type
		link        string // Link header, if any
		manifests   []v1.Descriptor
	}
	nextLink := func(i int) string {
		return fmt.Sprintf(`<%s?page=%d>; rel="next"`, path, i)
	}

	// newRegistry starts a registry that serves only the given pages. Any
	// other request, including one for the referrers fallback tag, fails the
	// test.
	newRegistry := func(t *testing.T, pages []page) *httptest.Server {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v2/" {
				return
			}
			if r.URL.Path != path {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if got := r.Header.Get("Accept"); got != string(types.OCIImageIndex) {
				t.Errorf("Accept = %q, want %q", got, types.OCIImageIndex)
			}
			i, _ := strconv.Atoi(r.URL.Query().Get("page"))
			p := pages[i]
			if p.link != "" {
				w.Header().Set("Link", p.link)
			}
			if p.status != 0 {
				w.WriteHeader(p.status)
				return
			}
			if p.contentType == "" {
				p.contentType = string(types.OCIImageIndex)
			}
			w.Header().Set("Content-Type", p.contentType)
			writeIndex(t, w, p.manifests)
		}))
		t.Cleanup(s.Close)
		return s
	}
	digest := func(t *testing.T, s *httptest.Server) name.Digest {
		u, err := url.Parse(s.URL)
		if err != nil {
			t.Fatal(err)
		}
		d, err := name.NewDigest(fmt.Sprintf("%s/repo@%s", u.Host, subject))
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	wantError := func(t *testing.T, s *httptest.Server, substr string) {
		_, err := remote.Referrers(digest(t, s))
		if err == nil {
			t.Fatalf("Referrers() succeeded, want error containing %q", substr)
		} else if !strings.Contains(err.Error(), substr) {
			t.Fatalf("Referrers() = %v, want error containing %q", err, substr)
		}
	}

	t.Run("follows Link headers", func(t *testing.T) {
		s := newRegistry(t, []page{
			{link: nextLink(1), manifests: referrers[0:2]},
			{link: nextLink(2), manifests: referrers[2:4]},
			{manifests: referrers[4:]},
		})

		index, err := remote.Referrers(digest(t, s))
		if err != nil {
			t.Fatal(err)
		}
		m, err := index.IndexManifest()
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(referrers, m.Manifests); d != "" {
			t.Errorf("referrers diff (-want,+got): %s", d)
		}

		// Filters apply to every page, not just the first.
		index, err = remote.Referrers(digest(t, s), remote.WithFilter("artifactType", referrers[4].ArtifactType))
		if err != nil {
			t.Fatal(err)
		}
		m, err = index.IndexManifest()
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(referrers[4:], m.Manifests); d != "" {
			t.Errorf("filtered referrers diff (-want,+got): %s", d)
		}
	})

	t.Run("refuses link to another host", func(t *testing.T) {
		other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("unexpected request to another host: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer other.Close()

		s := newRegistry(t, []page{
			{link: fmt.Sprintf(`<%s%s?page=1>; rel="next"`, other.URL, path), manifests: referrers[0:2]},
			{manifests: referrers[2:]},
		})
		wantError(t, s, "SSRF")
	})

	t.Run("refuses a link to a page already fetched", func(t *testing.T) {
		s := newRegistry(t, []page{
			{link: nextLink(1), manifests: referrers[0:2]},
			{link: nextLink(1), manifests: referrers[2:]},
		})
		wantError(t, s, "pagination loop")
	})

	t.Run("propagates error from a later page", func(t *testing.T) {
		// A 403 is not retried by the default transport, unlike a 5xx.
		s := newRegistry(t, []page{
			{link: nextLink(1), manifests: referrers[0:2]},
			{status: http.StatusForbidden},
		})

		_, err := remote.Referrers(digest(t, s))
		var terr *transport.Error
		if !errors.As(err, &terr) || terr.StatusCode != http.StatusForbidden {
			t.Fatalf("Referrers() = %v, want status %d", err, http.StatusForbidden)
		}
	})

	t.Run("rejects a later page that is not an index", func(t *testing.T) {
		// The body still parses as an empty index, so only the Content-Type
		// tells this apart from a final page with no referrers.
		s := newRegistry(t, []page{
			{link: nextLink(1), manifests: referrers[0:2]},
			{contentType: "application/json"},
		})
		wantError(t, s, "Content-Type")
	})

	t.Run("ignores Link headers outside the Referrers API", func(t *testing.T) {
		// A registry without the Referrers API. Both its 404 and the fallback
		// tag manifest carry a Link header, which must not be followed.
		fallback := "/v2/repo/manifests/" + strings.Replace(subject, ":", "-", 1)
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/v2/":
			case path:
				w.Header().Set("Link", nextLink(1))
				w.WriteHeader(http.StatusNotFound)
			case fallback:
				w.Header().Set("Link", nextLink(1))
				w.Header().Set("Content-Type", string(types.OCIImageIndex))
				writeIndex(t, w, referrers[0:2])
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		t.Cleanup(s.Close)

		index, err := remote.Referrers(digest(t, s))
		if err != nil {
			t.Fatal(err)
		}
		m, err := index.IndexManifest()
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(referrers[0:2], m.Manifests); d != "" {
			t.Errorf("referrers diff (-want,+got): %s", d)
		}
	})
}
