// Copyright 2026 Google LLC All Rights Reserved.
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
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
)

func TestPullerRetriesFailedInitialization(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()

	ref := registryReference(t, s.URL)
	if err := Write(ref, empty.Image); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	puller, err := NewPuller()
	if err != nil {
		t.Fatalf("NewPuller() = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := puller.Head(ctx, ref); !errors.Is(err, context.Canceled) {
		t.Fatalf("Head() = %v, want context.Canceled", err)
	}

	if _, err := puller.Head(context.Background(), ref); err != nil {
		t.Fatalf("Head() after canceled initialization = %v", err)
	}
}

func TestPusherRetriesFailedInitialization(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()

	ref := registryReference(t, s.URL)
	pusher, err := NewPusher()
	if err != nil {
		t.Fatalf("NewPusher() = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pusher.Push(ctx, ref, empty.Image); !errors.Is(err, context.Canceled) {
		t.Fatalf("Push() = %v, want context.Canceled", err)
	}

	if err := pusher.Push(context.Background(), ref, empty.Image); err != nil {
		t.Fatalf("Push() after canceled initialization = %v", err)
	}
}

func registryReference(t *testing.T, registryURL string) name.Reference {
	t.Helper()
	u, err := url.Parse(registryURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) = %v", registryURL, err)
	}
	ref, err := name.NewTag(fmt.Sprintf("%s/retry/init:latest", u.Host))
	if err != nil {
		t.Fatalf("name.NewTag() = %v", err)
	}
	return ref
}
