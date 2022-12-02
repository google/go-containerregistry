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

package gcrane

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/internal/retry"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type fakeXCR struct {
	h     http.Handler
	repos map[string]google.Tags
	t     *testing.T
}

func (xcr *fakeXCR) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	xcr.t.Logf("%s %s", r.Method, r.URL)
	if strings.HasPrefix(r.URL.Path, "/v2/") && strings.HasSuffix(r.URL.Path, "/tags/list") {
		repo := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v2/"), "/tags/list")
		if tags, ok := xcr.repos[repo]; !ok {
			w.WriteHeader(http.StatusNotFound)
		} else {
			xcr.t.Logf("%+v", tags)
			if err := json.NewEncoder(w).Encode(tags); err != nil {
				xcr.t.Fatal(err)
			}
		}
	} else {
		xcr.h.ServeHTTP(w, r)
	}
}

func newFakeXCR(t *testing.T) *fakeXCR {
	h := registry.New()
	return &fakeXCR{h: h, t: t}
}

func (xcr *fakeXCR) setRefs(stuff map[name.Reference]partial.Describable) error {
	repos := make(map[string]google.Tags)

	for ref, thing := range stuff {
		repo := ref.Context().RepositoryStr()
		tags, ok := repos[repo]
		if !ok {
			tags = google.Tags{
				Name:     repo,
				Children: []string{},
			}
		}

		// Populate the "child" field.
		for parentPath := repo; parentPath != "."; parentPath = path.Dir(parentPath) {
			child, parent := path.Base(parentPath), path.Dir(parentPath)
			tags, ok := repos[parent]
			if !ok {
				tags = google.Tags{}
			}
			for _, c := range repos[parent].Children {
				if c == child {
					break
				}
			}
			tags.Children = append(tags.Children, child)
			repos[parent] = tags
		}

		// Populate the "manifests" and "tags" field.
		d, err := thing.Digest()
		if err != nil {
			return err
		}
		mt, err := thing.MediaType()
		if err != nil {
			return err
		}
		if tags.Manifests == nil {
			tags.Manifests = make(map[string]google.ManifestInfo)
		}
		mi, ok := tags.Manifests[d.String()]
		if !ok {
			mi = google.ManifestInfo{
				MediaType: string(mt),
				Tags:      []string{},
			}
		}
		if tag, ok := ref.(name.Tag); ok {
			tags.Tags = append(tags.Tags, tag.Identifier())
			mi.Tags = append(mi.Tags, tag.Identifier())
		}
		tags.Manifests[d.String()] = mi
		repos[repo] = tags
	}
	xcr.repos = repos
	return nil
}

func TestCopy(t *testing.T) {
	logs.Warn.SetOutput(os.Stderr)
	xcr := newFakeXCR(t)
	s := httptest.NewServer(xcr)
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	src := path.Join(u.Host, "test/gcrane")
	dst := path.Join(u.Host, "test/gcrane/copy")

	oneTag, err := random.Image(1024, 5)
	if err != nil {
		t.Fatal(err)
	}
	twoTags, err := random.Image(1024, 5)
	if err != nil {
		t.Fatal(err)
	}
	noTags, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}

	latestRef, err := name.ParseReference(src)
	if err != nil {
		t.Fatal(err)
	}
	oneTagRef := latestRef.Context().Tag("bar")

	d, err := noTags.Digest()
	if err != nil {
		t.Fatal(err)
	}
	noTagsRef := latestRef.Context().Digest(d.String())
	fooRef := latestRef.Context().Tag("foo")

	// Populate this after we create it so we know the hostname.
	if err := xcr.setRefs(map[name.Reference]partial.Describable{
		oneTagRef: oneTag,
		latestRef: twoTags,
		fooRef:    twoTags,
		noTagsRef: noTags,
	}); err != nil {
		t.Fatal(err)
	}

	if err := remote.Write(latestRef, twoTags); err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(fooRef, twoTags); err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(oneTagRef, oneTag); err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(noTagsRef, noTags); err != nil {
		t.Fatal(err)
	}

	if err := Copy(src, dst); err != nil {
		t.Fatal(err)
	}

	if err := CopyRepository(context.Background(), src, dst); err != nil {
		t.Fatal(err)
	}
}

func TestRename(t *testing.T) {
	c := copier{
		srcRepo: name.MustParseReference("registry.example.com/foo").Context(),
		dstRepo: name.MustParseReference("registry.example.com/bar").Context(),
	}

	got, err := c.rename(name.MustParseReference("registry.example.com/foo/sub/repo").Context())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := name.MustParseReference("registry.example.com/bar/sub/repo").Context()

	if want.String() != got.String() {
		t.Errorf("%s != %s", want, got)
	}
}

func TestSubtractStringLists(t *testing.T) {
	cases := []struct {
		minuend    []string
		subtrahend []string
		result     []string
	}{{
		minuend:    []string{"a", "b", "c"},
		subtrahend: []string{"a"},
		result:     []string{"b", "c"},
	}, {
		minuend:    []string{"a", "a", "a"},
		subtrahend: []string{"a", "b"},
		result:     []string{},
	}, {
		minuend:    []string{},
		subtrahend: []string{"a", "b"},
		result:     []string{},
	}, {
		minuend:    []string{"a", "b"},
		subtrahend: []string{},
		result:     []string{"a", "b"},
	}}

	for _, tc := range cases {
		want, got := tc.result, subtractStringLists(tc.minuend, tc.subtrahend)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("subtracting string lists: %v - %v: (-want +got)\n%s", tc.minuend, tc.subtrahend, diff)
		}
	}
}

func TestDiffImages(t *testing.T) {
	cases := []struct {
		want map[string]google.ManifestInfo
		have map[string]google.ManifestInfo
		need map[string]google.ManifestInfo
	}{{
		// Have everything we need.
		want: map[string]google.ManifestInfo{
			"a": {
				Tags: []string{"b", "c"},
			},
		},
		have: map[string]google.ManifestInfo{
			"a": {
				Tags: []string{"b", "c"},
			},
		},
		need: map[string]google.ManifestInfo{},
	}, {
		// Missing image a.
		want: map[string]google.ManifestInfo{
			"a": {
				Tags: []string{"b", "c", "d"},
			},
		},
		have: map[string]google.ManifestInfo{},
		need: map[string]google.ManifestInfo{
			"a": {
				Tags: []string{"b", "c", "d"},
			},
		},
	}, {
		// Missing tags "b" and "d"
		want: map[string]google.ManifestInfo{
			"a": {
				Tags: []string{"b", "c", "d"},
			},
		},
		have: map[string]google.ManifestInfo{
			"a": {
				Tags: []string{"c"},
			},
		},
		need: map[string]google.ManifestInfo{
			"a": {
				Tags: []string{"b", "d"},
			},
		},
	}, {
		// Make sure all properties get copied over.
		want: map[string]google.ManifestInfo{
			"a": {
				Size:      123,
				MediaType: string(types.DockerManifestSchema2),
				Created:   time.Date(1992, time.January, 7, 6, 40, 00, 5e8, time.UTC),
				Uploaded:  time.Date(2018, time.November, 29, 4, 13, 30, 5e8, time.UTC),
				Tags:      []string{"b", "c", "d"},
			},
		},
		have: map[string]google.ManifestInfo{},
		need: map[string]google.ManifestInfo{
			"a": {
				Size:      123,
				MediaType: string(types.DockerManifestSchema2),
				Created:   time.Date(1992, time.January, 7, 6, 40, 00, 5e8, time.UTC),
				Uploaded:  time.Date(2018, time.November, 29, 4, 13, 30, 5e8, time.UTC),
				Tags:      []string{"b", "c", "d"},
			},
		},
	}}

	for _, tc := range cases {
		want, got := tc.need, diffImages(tc.want, tc.have)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("diffing images: %v - %v: (-want +got)\n%s", tc.want, tc.have, diff)
		}
	}
}

// Test that our backoff works the way we expect.
func TestBackoff(t *testing.T) {
	backoff := GCRBackoff()

	if d := backoff.Step(); d > 10*time.Second {
		t.Errorf("Duration too long: %v", d)
	}
	if d := backoff.Step(); d > 100*time.Second {
		t.Errorf("Duration too long: %v", d)
	}
	if d := backoff.Step(); d > 1000*time.Second {
		t.Errorf("Duration too long: %v", d)
	}
	if s := backoff.Steps; s != 0 {
		t.Errorf("backoff.Steps should be 0, got %d", s)
	}
}

func TestErrors(t *testing.T) {
	if hasStatusCode(nil, http.StatusOK) {
		t.Fatal("nil error should not have any status code")
	}
	if !hasStatusCode(&transport.Error{StatusCode: http.StatusOK}, http.StatusOK) {
		t.Fatal("200 should be 200")
	}
	if hasStatusCode(&transport.Error{StatusCode: http.StatusOK}, http.StatusNotFound) {
		t.Fatal("200 should not be 404")
	}

	if isServerError(nil) {
		t.Fatal("nil should not be server error")
	}
	if isServerError(fmt.Errorf("i am a string")) {
		t.Fatal("string should not be server error")
	}
	if !isServerError(&transport.Error{StatusCode: http.StatusServiceUnavailable}) {
		t.Fatal("503 should be server error")
	}
	if isServerError(&transport.Error{StatusCode: http.StatusTooManyRequests}) {
		t.Fatal("429 should not be server error")
	}
}

func TestRetryErrors(t *testing.T) {
	// We log a warning during retries, so we can tell if something retried by checking logs.Warn.
	var b bytes.Buffer
	logs.Warn.SetOutput(&b)

	err := backoffErrors(retry.Backoff{
		Duration: 1 * time.Millisecond,
		Steps:    3,
	}, func() error {
		return &transport.Error{StatusCode: http.StatusTooManyRequests}
	})

	if err == nil {
		t.Fatal("backoffErrors should return internal err, got nil")
	}
	var terr *transport.Error
	if !errors.As(err, &terr) {
		t.Fatalf("backoffErrors should return internal err, got different error: %v", err)
	} else if terr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("backoffErrors should return internal err, got different status code: %v", terr.StatusCode)
	}

	if b.Len() == 0 {
		t.Fatal("backoffErrors didn't log to logs.Warn")
	}
}

func TestBadInputs(t *testing.T) {
	t.Parallel()
	invalid := "@@@@@@"

	// Create a valid image reference that will fail with not found.
	s := httptest.NewServer(http.NotFoundHandler())
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	valid404 := fmt.Sprintf("%s/some/image", u.Host)

	ctx := context.Background()

	for _, tc := range []struct {
		desc string
		err  error
	}{
		{"Copy(invalid, invalid)", Copy(invalid, invalid)},
		{"Copy(404, invalid)", Copy(valid404, invalid)},
		{"Copy(404, 404)", Copy(valid404, valid404)},
		{"CopyRepository(invalid, invalid)", CopyRepository(ctx, invalid, invalid)},
		{"CopyRepository(404, invalid)", CopyRepository(ctx, valid404, invalid)},
		{"CopyRepository(404, 404)", CopyRepository(ctx, valid404, valid404, WithJobs(1))},
	} {
		if tc.err == nil {
			t.Errorf("%s: expected err, got nil", tc.desc)
		}
	}
}
