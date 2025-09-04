// Copyright 2019 Google LLC All Rights Reserved.
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

package crane_test

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/compare"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func TestTagSingle(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	src := fmt.Sprintf("%s/test/tag-single", u.Host)
	tagName := "single-tag"

	// Create and push a test image.
	img, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}

	if err := crane.Push(img, src); err != nil {
		t.Fatal(err)
	}

	// Test single tag using string (backward compatibility).
	if err := crane.Tag(src, tagName); err != nil {
		t.Fatalf("Tag single string failed: %v", err)
	}

	// Verify the tag was created.
	tagged, err := crane.Pull(fmt.Sprintf("%s:%s", src, tagName))
	if err != nil {
		t.Fatalf("Failed to pull tagged image: %v", err)
	}

	if err := compare.Images(img, tagged); err != nil {
		t.Fatalf("Tagged image differs from original: %v", err)
	}

	// Verify tag exists in listing.
	tags, err := crane.ListTags(src)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, tag := range tags {
		if tag == tagName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Tag %q not found in listing: %v", tagName, tags)
	}
}

func TestTagMultiple(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	src := fmt.Sprintf("%s/test/tag-multiple", u.Host)
	tagNames := []string{"tag1", "tag2", "tag3"}

	// Create and push a test image.
	img, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}

	if err := crane.Push(img, src); err != nil {
		t.Fatal(err)
	}

	// Test multiple tags using slice.
	if err := crane.TagMultiple(src, tagNames); err != nil {
		t.Fatalf("Tag multiple strings failed: %v", err)
	}

	// Verify all tags were created and point to the same image.
	for _, tagName := range tagNames {
		tagged, err := crane.Pull(fmt.Sprintf("%s:%s", src, tagName))
		if err != nil {
			t.Fatalf("Failed to pull tagged image %s: %v", tagName, err)
		}

		if err := compare.Images(img, tagged); err != nil {
			t.Fatalf("Tagged image %s differs from original: %v", tagName, err)
		}
	}

	// Verify all tags exist in listing.
	tags, err := crane.ListTags(src)
	if err != nil {
		t.Fatal(err)
	}

	for _, expectedTag := range tagNames {
		found := false
		for _, tag := range tags {
			if tag == expectedTag {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Tag %q not found in listing: %v", expectedTag, tags)
		}
	}

	// Verify the total number of tags (original 'latest' + our 3 tags).
	expectedCount := len(tagNames) + 1 // +1 for 'latest'
	if len(tags) != expectedCount {
		t.Fatalf("Expected %d tags, got %d: %v", expectedCount, len(tags), tags)
	}
}

func TestTagEmpty(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	src := fmt.Sprintf("%s/test/tag-empty", u.Host)

	// Create and push a test image.
	img, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}

	if err := crane.Push(img, src); err != nil {
		t.Fatal(err)
	}

	// Test empty slice - should be a no-op.
	emptyTags := []string{}
	if err := crane.TagMultiple(src, emptyTags); err != nil {
		t.Fatalf("Tag with empty slice failed: %v", err)
	}

	// Verify no additional tags were created (should still just have 'latest').
	tags, err := crane.ListTags(src)
	if err != nil {
		t.Fatal(err)
	}

	if len(tags) != 1 || tags[0] != "latest" {
		t.Fatalf("Expected only 'latest' tag, got: %v", tags)
	}
}

func TestTagInvalidType(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	src := fmt.Sprintf("%s/test/tag-invalid", u.Host)

	// Create and push a test image.
	img, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}

	if err := crane.Push(img, src); err != nil {
		t.Fatal(err)
	}

	// Test that TagMultiple properly validates input (nil slice).
	if err := crane.TagMultiple(src, nil); err != nil {
		// This should succeed as a no-op for nil slice
		t.Fatalf("TagMultiple with nil slice should succeed: %v", err)
	}
}

func TestTagWithInvalidImageRef(t *testing.T) {
	invalidRef := "/dev/null/@@@@@@"

	// Test single tag with invalid reference.
	if err := crane.Tag(invalidRef, "tag"); err == nil {
		t.Fatal("Expected error for invalid image reference, got nil")
	}

	// Test multiple tags with invalid reference.
	if err := crane.TagMultiple(invalidRef, []string{"tag1", "tag2"}); err == nil {
		t.Fatal("Expected error for invalid image reference with multiple tags, got nil")
	}
}

func TestTagWithNonExistentImage(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	nonExistent := fmt.Sprintf("%s/does/not/exist", u.Host)

	// Test single tag with non-existent image.
	if err := crane.Tag(nonExistent, "tag"); err == nil {
		t.Fatal("Expected error for non-existent image, got nil")
	}

	// Test multiple tags with non-existent image.
	if err := crane.TagMultiple(nonExistent, []string{"tag1", "tag2"}); err == nil {
		t.Fatal("Expected error for non-existent image with multiple tags, got nil")
	}
}

func TestTagFailurePartialApplication(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	src := fmt.Sprintf("%s/test/tag-failure", u.Host)

	// Create and push a test image.
	img, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}

	if err := crane.Push(img, src); err != nil {
		t.Fatal(err)
	}

	// Test multiple tags where one is invalid (contains invalid characters).
	invalidTags := []string{"valid-tag", "invalid/tag/with/slashes", "another-valid-tag"}
	
	err = crane.TagMultiple(src, invalidTags)
	if err == nil {
		t.Fatal("Expected error for invalid tag name, got nil")
	}

	// Check that the error message includes information about successful tags.
	expectedErrMsg := "successfully tagged with [valid-tag]"
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Fatalf("Expected error to contain %q, got: %v", expectedErrMsg, err.Error())
	}

	// Verify that partial tags were created (non-atomic behavior).
	tags, err := crane.ListTags(src)
	if err != nil {
		t.Fatal(err)
	}

	// Should have 'latest' and 'valid-tag' (the one that succeeded before failure).
	expectedTags := []string{"latest", "valid-tag"}
	if len(tags) != len(expectedTags) {
		t.Fatalf("Expected %d tags after partial tagging, got %d: %v", len(expectedTags), len(tags), tags)
	}

	// Check that valid-tag exists.
	found := false
	for _, tag := range tags {
		if tag == "valid-tag" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected 'valid-tag' to exist after partial tagging, got: %v", tags)
	}
}

func TestTagIntegrationWithRemote(t *testing.T) {
	// Set up a fake registry.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	src := fmt.Sprintf("%s/test/tag-integration", u.Host)

	// Create and push a test image.
	img, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}

	ref, err := name.ParseReference(src)
	if err != nil {
		t.Fatal(err)
	}

	if err := remote.Write(ref, img); err != nil {
		t.Fatal(err)
	}

	// Test that our TagMultiple function works with images pushed via remote.Write.
	testTags := []string{"integration-test-1", "integration-test-2"}
	
	if err := crane.TagMultiple(src, testTags); err != nil {
		t.Fatalf("TagMultiple integration test failed: %v", err)
	}

	// Verify tags were created correctly.
	for _, tagName := range testTags {
		tagged, err := crane.Pull(fmt.Sprintf("%s:%s", src, tagName))
		if err != nil {
			t.Fatalf("Failed to pull integration test tagged image %s: %v", tagName, err)
		}

		if err := compare.Images(img, tagged); err != nil {
			t.Fatalf("Integration test tagged image %s differs from original: %v", tagName, err)
		}
	}
}