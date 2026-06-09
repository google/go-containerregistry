// Copyright 2024 Google LLC All Rights Reserved.
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
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestWithManifestHandler(t *testing.T) {
	reg := registry.New(registry.WithManifestHandler(registry.NewInMemoryManifestHandler()))
	srv := httptest.NewServer(reg)
	defer srv.Close()

	ref, err := name.ParseReference(strings.TrimPrefix(srv.URL, "http://") + "/foo/bar:latest")
	if err != nil {
		t.Fatal(err)
	}
	img, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatalf("remote.Write: %v", err)
	}
	pulled, err := remote.Image(ref)
	if err != nil {
		t.Fatalf("remote.Image: %v", err)
	}
	if err := validate.Image(pulled); err != nil {
		t.Fatalf("validate.Image: %v", err)
	}
}

// sharedStore is a minimal ManifestHandler that keeps manifests in a store that
// can outlive a single registry instance, demonstrating the persistence use
// case behind WithManifestHandler.
type sharedStore struct {
	lock sync.Mutex
	// repo -> ref(tag|digest) -> manifest
	m map[string]map[string]registry.Manifest
}

func newSharedStore() *sharedStore {
	return &sharedStore{m: map[string]map[string]registry.Manifest{}}
}

func (s *sharedStore) Get(_ context.Context, repo, target string) (*registry.Manifest, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	c, ok := s.m[repo]
	if !ok {
		return nil, registry.ErrNameUnknown
	}
	mf, ok := c[target]
	if !ok {
		return nil, registry.ErrNotFound
	}
	return &mf, nil
}

func (s *sharedStore) Put(_ context.Context, repo, target, digest string, manifest *registry.Manifest) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.m[repo]; !ok {
		s.m[repo] = map[string]registry.Manifest{}
	}
	s.m[repo][digest] = *manifest
	s.m[repo][target] = *manifest
	return nil
}

func (s *sharedStore) Delete(_ context.Context, repo, target string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.m[repo]; !ok {
		return registry.ErrNameUnknown
	}
	if _, ok := s.m[repo][target]; !ok {
		return registry.ErrNotFound
	}
	delete(s.m[repo], target)
	return nil
}

// TestManifestHandlerPersists writes an image to one registry instance, then
// stands up a fresh instance backed by the same store (simulating a restart)
// and confirms the manifest is still served. Blobs are shared the same way via
// WithBlobHandler so the pull validates end to end.
func TestManifestHandlerPersists(t *testing.T) {
	store := newSharedStore()
	blobs := registry.NewInMemoryBlobHandler()

	first := httptest.NewServer(registry.New(
		registry.WithManifestHandler(store),
		registry.WithBlobHandler(blobs),
	))
	pushRef, err := name.ParseReference(strings.TrimPrefix(first.URL, "http://") + "/foo/bar:latest")
	if err != nil {
		t.Fatal(err)
	}
	img, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(pushRef, img); err != nil {
		t.Fatalf("remote.Write: %v", err)
	}
	first.Close()

	// "Restart": new registry instance, same backing store.
	second := httptest.NewServer(registry.New(
		registry.WithManifestHandler(store),
		registry.WithBlobHandler(blobs),
	))
	defer second.Close()

	pullRef, err := name.ParseReference(strings.TrimPrefix(second.URL, "http://") + "/foo/bar:latest")
	if err != nil {
		t.Fatal(err)
	}
	pulled, err := remote.Image(pullRef)
	if err != nil {
		t.Fatalf("remote.Image after restart: %v", err)
	}
	if err := validate.Image(pulled); err != nil {
		t.Fatalf("validate.Image after restart: %v", err)
	}
}
