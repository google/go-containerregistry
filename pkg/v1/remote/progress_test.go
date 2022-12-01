// Copyright 2021 Google LLC All Rights Reserved.
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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestWriteLayer_Progress(t *testing.T) {
	l, err := random.Layer(1000, types.OCIUncompressedLayer)
	if err != nil {
		t.Fatal(err)
	}
	c := make(chan v1.Update, 200)

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	dst := fmt.Sprintf("%s/test/progress/upload", u.Host)
	ref, err := name.ParseReference(dst)
	if err != nil {
		t.Fatal(err)
	}

	if err := WriteLayer(ref.Context(), l, WithProgress(c)); err != nil {
		t.Fatalf("WriteLayer: %v", err)
	}
	if err := checkUpdates(c); err != nil {
		t.Fatal(err)
	}
}

// TestWriteLayer_Progress_Exists tests progress reporting behavior when the
// layer already exists in the registry, so writes are skipped, but progress
// should still be reported in one update.
func TestWriteLayer_Progress_Exists(t *testing.T) {
	l, err := random.Layer(1000, types.OCILayer)
	if err != nil {
		t.Fatal(err)
	}
	c := make(chan v1.Update, 200)

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	dst := fmt.Sprintf("%s/test/progress/upload", u.Host)
	ref, err := name.ParseReference(dst)
	if err != nil {
		t.Fatal(err)
	}

	// Write the layer, so we can get updates when we write it again.
	if err := WriteLayer(ref.Context(), l); err != nil {
		t.Fatalf("WriteLayer: %v", err)
	}
	if err := WriteLayer(ref.Context(), l, WithProgress(c)); err != nil {
		t.Fatalf("WriteLayer: %v", err)
	}
	if err := checkUpdates(c); err != nil {
		t.Fatal(err)
	}
}

func TestWrite_Progress(t *testing.T) {
	img, err := random.Image(1000, 5)
	if err != nil {
		t.Fatal(err)
	}
	c := make(chan v1.Update, 200)

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	dst := fmt.Sprintf("%s/test/progress/upload", u.Host)
	ref, err := name.ParseReference(dst)
	if err != nil {
		t.Fatal(err)
	}

	if err := Write(ref, img, WithProgress(c)); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := checkUpdates(c); err != nil {
		t.Fatal(err)
	}
}

// An image with multiple identical layers is handled correctly.
func TestWrite_Progress_DedupeLayers(t *testing.T) {
	img := empty.Image
	for i := 0; i < 10; i++ {
		l, err := random.Layer(1000, types.OCILayer)
		if err != nil {
			t.Fatal(err)
		}

		img, err = mutate.AppendLayers(img, l)
		if err != nil {
			t.Fatal(err)
		}
	}

	c := make(chan v1.Update, 200)

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	dst := fmt.Sprintf("%s/test/progress/upload", u.Host)
	ref, err := name.ParseReference(dst)
	if err != nil {
		t.Fatal(err)
	}

	if err := Write(ref, img, WithProgress(c)); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := checkUpdates(c); err != nil {
		t.Fatal(err)
	}
}

func TestWriteIndex_Progress(t *testing.T) {
	idx, err := random.Index(1000, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	c := make(chan v1.Update, 200)

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	dst := fmt.Sprintf("%s/test/progress/upload", u.Host)
	ref, err := name.ParseReference(dst)
	if err != nil {
		t.Fatal(err)
	}

	if err := WriteIndex(ref, idx, WithProgress(c)); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}

	if err := checkUpdates(c); err != nil {
		t.Fatal(err)
	}
}

func TestMultiWrite_Progress(t *testing.T) {
	idx, err := random.Index(1000, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	c := make(chan v1.Update, 1000)

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := name.ParseReference(fmt.Sprintf("%s/test/progress/upload", u.Host))
	if err != nil {
		t.Fatal(err)
	}
	ref2, err := name.ParseReference(fmt.Sprintf("%s/test/progress/upload:again", u.Host))
	if err != nil {
		t.Fatal(err)
	}

	if err := MultiWrite(map[name.Reference]Taggable{
		ref:  idx,
		ref2: idx,
	}, WithProgress(c)); err != nil {
		t.Fatalf("MultiWrite: %v", err)
	}

	if err := checkUpdates(c); err != nil {
		t.Fatal(err)
	}
}

func TestMultiWrite_Progress_Retry(t *testing.T) {
	idx, err := random.Index(1000, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	c := make(chan v1.Update, 1000)

	// Set up a fake registry.
	handler := registry.New()
	numOfInternalServerErrors := 0
	var mu sync.Mutex
	registryThatFailsOnFirstUpload := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		mu.Lock()
		defer mu.Unlock()
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

	ref, err := name.ParseReference(fmt.Sprintf("%s/test/progress/upload", u.Host))
	if err != nil {
		t.Fatal(err)
	}
	ref2, err := name.ParseReference(fmt.Sprintf("%s/test/progress/upload:again", u.Host))
	if err != nil {
		t.Fatal(err)
	}

	if err := MultiWrite(map[name.Reference]Taggable{
		ref:  idx,
		ref2: idx,
	}, WithProgress(c), WithRetryBackoff(fastBackoff)); err != nil {
		t.Fatalf("MultiWrite: %v", err)
	}

	if err := checkUpdates(c); err != nil {
		t.Fatal(err)
	}
}

func TestWriteLayer_Progress_Retry(t *testing.T) {
	l, err := random.Layer(100000, types.OCIUncompressedLayer)
	if err != nil {
		t.Fatal(err)
	}
	c := make(chan v1.Update, 200)

	// Set up a fake registry.
	handler := registry.New()

	numOfInternalServerErrors := 0
	registryThatFailsOnFirstUpload := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPatch && strings.Contains(request.URL.Path, "upload/blobs/uploads") && numOfInternalServerErrors < 1 {
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
	dst := fmt.Sprintf("%s/test/progress/upload", u.Host)
	ref, err := name.ParseReference(dst)
	if err != nil {
		t.Fatal(err)
	}

	if err := WriteLayer(ref.Context(), l, WithProgress(c), WithRetryBackoff(fastBackoff)); err != nil {
		t.Fatalf("WriteLayer: %v", err)
	}

	everyUpdate := []v1.Update{}
	for update := range c {
		everyUpdate = append(everyUpdate, update)
	}

	if diff := cmp.Diff(everyUpdate, []v1.Update{
		{Total: 101921, Complete: 32768},
		{Total: 101921, Complete: 65536},
		{Total: 101921, Complete: 98304},
		{Total: 101921, Complete: 101921},
		// retry results in the same messages sent to the updates channel
		{Total: 101921, Complete: 0},
		{Total: 101921, Complete: 32768},
		{Total: 101921, Complete: 65536},
		{Total: 101921, Complete: 98304},
		{Total: 101921, Complete: 101921},
	}); diff != "" {
		t.Errorf("received updates (-want +got) = %s", diff)
	}
}

func TestWriteLayer_Progress_Error(t *testing.T) {
	l, err := random.Layer(100000, types.OCIUncompressedLayer)
	if err != nil {
		t.Fatal(err)
	}
	c := make(chan v1.Update, 200)

	// Set up a fake registry.
	handler := registry.New()
	registryThatAlwaysFails := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPatch && strings.Contains(request.URL.Path, "blobs/uploads") {
			responseWriter.WriteHeader(403)
		}
		handler.ServeHTTP(responseWriter, request)
	})

	s := httptest.NewServer(registryThatAlwaysFails)
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	dst := fmt.Sprintf("%s/test/progress/upload", u.Host)
	ref, err := name.ParseReference(dst)
	if err != nil {
		t.Fatal(err)
	}

	if err := WriteLayer(ref.Context(), l, WithProgress(c)); err == nil {
		t.Errorf("WriteLayer: wanted error, got nil")
	}

	everyUpdate := []v1.Update{}
	for update := range c {
		everyUpdate = append(everyUpdate, update)
	}

	if diff := cmp.Diff(everyUpdate[:len(everyUpdate)-1], []v1.Update{
		{Total: 101921, Complete: 32768},
		{Total: 101921, Complete: 65536},
		{Total: 101921, Complete: 98304},
		{Total: 101921, Complete: 101921},
		// retry results in the same messages sent to the updates channel
		{Total: 101921, Complete: 0},
	}); diff != "" {
		t.Errorf("received updates (-want +got) = %s", diff)
	}
	if everyUpdate[len(everyUpdate)-1].Error == nil {
		t.Errorf("Last update had nil error")
	}
}

func TestWrite_Progress_WithNonDistributableLayer_AndIncludeNonDistributableLayersOption(t *testing.T) {
	ociLayer, err := random.Layer(1000, types.OCILayer)
	if err != nil {
		t.Fatal(err)
	}

	nonDistributableLayer, err := random.Layer(1000, types.OCIRestrictedLayer)
	if err != nil {
		t.Fatal(err)
	}

	img, err := mutate.AppendLayers(empty.Image, ociLayer, nonDistributableLayer)
	if err != nil {
		t.Fatal(err)
	}

	c := make(chan v1.Update, 200)

	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	dst := fmt.Sprintf("%s/test/progress/upload", u.Host)
	ref, err := name.ParseReference(dst)
	if err != nil {
		t.Fatal(err)
	}

	if err := Write(ref, img, WithProgress(c), WithNondistributable); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := checkUpdates(c); err != nil {
		t.Fatal(err)
	}
}

// checkUpdates checks that updates show steady progress toward a total, and
// don't describe errors.
func checkUpdates(updates <-chan v1.Update) error {
	var high, total int64
	for u := range updates {
		if u.Error != nil {
			return u.Error
		}

		if u.Total == 0 {
			return errors.New("saw zero total")
		}

		if total == 0 {
			total = u.Total
		} else if u.Total != total {
			return fmt.Errorf("total changed: was %d, saw %d", total, u.Total)
		}

		if u.Complete < high {
			return fmt.Errorf("saw progress revert: was high of %d, saw %d", high, u.Complete)
		}
		high = u.Complete
	}

	if high > total {
		return fmt.Errorf("final progress (%d) exceeded total (%d) by %d", high, total, high-total)
	} else if high < total {
		return fmt.Errorf("final progress (%d) did not reach total (%d) by %d", high, total, total-high)
	}

	return nil
}
