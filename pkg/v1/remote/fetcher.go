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

package remote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/internal/redact"
	"github.com/google/go-containerregistry/internal/verify"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

const (
	kib           = 1024
	mib           = 1024 * kib
	manifestLimit = 100 * mib
)

// fetcher implements methods for reading from a registry.
type fetcher struct {
	target resource
	client *http.Client
}

func makeFetcher(ctx context.Context, target resource, o *options) (*fetcher, error) {
	auth := o.auth
	if o.keychain != nil {
		kauth, err := authn.Resolve(ctx, o.keychain, target)
		if err != nil {
			return nil, err
		}
		auth = kauth
	}

	reg, ok := target.(name.Registry)
	if !ok {
		repo, ok := target.(name.Repository)
		if !ok {
			return nil, fmt.Errorf("unexpected resource: %T", target)
		}
		reg = repo.Registry
	}

	tr, err := transport.NewWithContext(ctx, reg, auth, o.transport, []string{target.Scope(transport.PullScope)})
	if err != nil {
		return nil, err
	}
	return &fetcher{
		target: target,
		client: &http.Client{Transport: tr},
	}, nil
}

func (f *fetcher) Do(req *http.Request) (*http.Response, error) {
	return f.client.Do(req)
}

type resource interface {
	Scheme() string
	RegistryStr() string
	Scope(string) string

	authn.Resource
}

// url returns a url.Url for the specified path in the context of this remote image reference.
func (f *fetcher) url(resource, identifier string) url.URL {
	u := url.URL{
		Scheme: f.target.Scheme(),
		Host:   f.target.RegistryStr(),
		// Default path if this is not a repository.
		Path: "/v2/_catalog",
	}
	if repo, ok := f.target.(name.Repository); ok {
		u.Path = fmt.Sprintf("/v2/%s/%s/%s", repo.RepositoryStr(), resource, identifier)
	}
	return u
}

func (f *fetcher) get(ctx context.Context, ref name.Reference, acceptable []types.MediaType, platform v1.Platform) (*Descriptor, error) {
	b, desc, err := f.fetchManifest(ctx, ref, acceptable)
	if err != nil {
		return nil, err
	}
	return &Descriptor{
		ref:        ref,
		ctx:        ctx,
		fetcher:    *f,
		Manifest:   b,
		Descriptor: *desc,
		platform:   platform,
	}, nil
}

func (f *fetcher) fetchManifest(ctx context.Context, ref name.Reference, acceptable []types.MediaType) ([]byte, *v1.Descriptor, error) {
	u := f.url("manifests", ref.Identifier())
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	accept := []string{}
	for _, mt := range acceptable {
		accept = append(accept, string(mt))
	}
	req.Header.Set("Accept", strings.Join(accept, ","))

	resp, err := f.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if err := transport.CheckError(resp, http.StatusOK); err != nil {
		return nil, nil, err
	}

	manifest, err := io.ReadAll(io.LimitReader(resp.Body, manifestLimit))
	if err != nil {
		return nil, nil, err
	}

	digest, size, err := v1.SHA256(bytes.NewReader(manifest))
	if err != nil {
		return nil, nil, err
	}

	mediaType := types.MediaType(resp.Header.Get("Content-Type"))
	contentDigest, err := v1.NewHash(resp.Header.Get("Docker-Content-Digest"))
	if err == nil && mediaType == types.DockerManifestSchema1Signed {
		// If we can parse the digest from the header, and it's a signed schema 1
		// manifest, let's use that for the digest to appease older registries.
		digest = contentDigest
	}

	// Validate the digest matches what we asked for, if pulling by digest.
	if dgst, ok := ref.(name.Digest); ok {
		if digest.String() != dgst.DigestStr() {
			return nil, nil, fmt.Errorf("manifest digest: %q does not match requested digest: %q for %q", digest, dgst.DigestStr(), ref)
		}
	}

	var artifactType string
	mf, _ := v1.ParseManifest(bytes.NewReader(manifest))
	// Failing to parse as a manifest should just be ignored.
	// The manifest might not be valid, and that's okay.
	if mf != nil && !mf.Config.MediaType.IsConfig() {
		artifactType = string(mf.Config.MediaType)
	}

	// Do nothing for tags; I give up.
	//
	// We'd like to validate that the "Docker-Content-Digest" header matches what is returned by the registry,
	// but so many registries implement this incorrectly that it's not worth checking.
	//
	// For reference:
	// https://github.com/GoogleContainerTools/kaniko/issues/298

	// Return all this info since we have to calculate it anyway.
	desc := v1.Descriptor{
		Digest:       digest,
		Size:         size,
		MediaType:    mediaType,
		ArtifactType: artifactType,
	}

	return manifest, &desc, nil
}

func (f *fetcher) headManifest(ctx context.Context, ref name.Reference, acceptable []types.MediaType) (*v1.Descriptor, error) {
	u := f.url("manifests", ref.Identifier())
	req, err := http.NewRequest(http.MethodHead, u.String(), nil)
	if err != nil {
		return nil, err
	}
	accept := []string{}
	for _, mt := range acceptable {
		accept = append(accept, string(mt))
	}
	req.Header.Set("Accept", strings.Join(accept, ","))

	resp, err := f.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := transport.CheckError(resp, http.StatusOK); err != nil {
		return nil, err
	}

	mth := resp.Header.Get("Content-Type")
	if mth == "" {
		return nil, fmt.Errorf("HEAD %s: response did not include Content-Type header", u.String())
	}
	mediaType := types.MediaType(mth)

	size := resp.ContentLength
	if size == -1 {
		return nil, fmt.Errorf("GET %s: response did not include Content-Length header", u.String())
	}

	dh := resp.Header.Get("Docker-Content-Digest")
	if dh == "" {
		return nil, fmt.Errorf("HEAD %s: response did not include Docker-Content-Digest header", u.String())
	}
	digest, err := v1.NewHash(dh)
	if err != nil {
		return nil, err
	}

	// Validate the digest matches what we asked for, if pulling by digest.
	if dgst, ok := ref.(name.Digest); ok {
		if digest.String() != dgst.DigestStr() {
			return nil, fmt.Errorf("manifest digest: %q does not match requested digest: %q for %q", digest, dgst.DigestStr(), ref)
		}
	}

	// Return all this info since we have to calculate it anyway.
	return &v1.Descriptor{
		Digest:    digest,
		Size:      size,
		MediaType: mediaType,
	}, nil
}

// contextKey is a type for context keys used in this package
type contextKey string

const resumeOffsetKey contextKey = "resumeOffset"
const resumeOffsetsKey contextKey = "resumeOffsets"

// WithResumeOffset returns a context with the resume offset set for a single blob
func WithResumeOffset(ctx context.Context, offset int64) context.Context {
	return context.WithValue(ctx, resumeOffsetKey, offset)
}

// WithResumeOffsets returns a context with resume offsets for multiple blobs (keyed by digest)
func WithResumeOffsets(ctx context.Context, offsets map[string]int64) context.Context {
	return context.WithValue(ctx, resumeOffsetsKey, offsets)
}

// getResumeOffset retrieves the resume offset from context for a given digest
func getResumeOffset(ctx context.Context, digest string) int64 {
	// First check if there's a specific offset for this digest
	if offsets, ok := ctx.Value(resumeOffsetsKey).(map[string]int64); ok {
		if offset, found := offsets[digest]; found && offset > 0 {
			return offset
		}
	}
	// Fall back to single offset (for fetchBlob)
	if offset, ok := ctx.Value(resumeOffsetKey).(int64); ok {
		return offset
	}
	return 0
}

func (f *fetcher) fetchBlob(ctx context.Context, size int64, h v1.Hash) (io.ReadCloser, error) {
	u := f.url("blobs", h.String())
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	// Check if we should resume from a specific offset
	resumeOffset := getResumeOffset(ctx, h.String())
	if resumeOffset > 0 {
		// Add Range header to resume download
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeOffset))
	}

	resp, err := f.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, redact.Error(err)
	}

	// Accept both 200 OK (full content) and 206 Partial Content (resumed)
	if resumeOffset > 0 {
		// If we requested a Range but got 200, the server doesn't support ranges
		// We'll have to download from scratch
		if resp.StatusCode == http.StatusOK {
			// Server doesn't support range requests, will download full content
			resumeOffset = 0
		}
	}

	if err := transport.CheckError(resp, http.StatusOK, http.StatusPartialContent); err != nil {
		resp.Body.Close()
		return nil, err
	}

	// For partial content (resumed downloads), we can't verify the hash on the stream
	// since we're only getting part of the file. The complete file will be verified
	// after all bytes are written to disk.
	if resumeOffset > 0 && resp.StatusCode == http.StatusPartialContent {
		// Verify Content-Length matches expected remaining size
		if hsize := resp.ContentLength; hsize != -1 {
			if size != verify.SizeUnknown {
				expectedRemaining := size - resumeOffset
				if hsize != expectedRemaining {
					resp.Body.Close()
					return nil, fmt.Errorf("GET %s: Content-Length header %d does not match expected remaining size %d", u.String(), hsize, expectedRemaining)
				}
			}
		}
		// Return the body without verification - we'll verify the complete file later
		return io.NopCloser(resp.Body), nil
	}

	// For full downloads, verify the stream
	// Do whatever we can with size validation
	if hsize := resp.ContentLength; hsize != -1 {
		if size == verify.SizeUnknown {
			size = hsize
		} else if hsize != size {
			resp.Body.Close()
			return nil, fmt.Errorf("GET %s: Content-Length header %d does not match expected size %d", u.String(), hsize, size)
		}
	}

	return verify.ReadCloser(resp.Body, size, h)
}

func (f *fetcher) headBlob(ctx context.Context, h v1.Hash) (*http.Response, error) {
	u := f.url("blobs", h.String())
	req, err := http.NewRequest(http.MethodHead, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, redact.Error(err)
	}

	if err := transport.CheckError(resp, http.StatusOK); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

func (f *fetcher) blobExists(ctx context.Context, h v1.Hash) (bool, error) {
	u := f.url("blobs", h.String())
	req, err := http.NewRequest(http.MethodHead, u.String(), nil)
	if err != nil {
		return false, err
	}

	resp, err := f.client.Do(req.WithContext(ctx))
	if err != nil {
		return false, redact.Error(err)
	}
	defer resp.Body.Close()

	if err := transport.CheckError(resp, http.StatusOK, http.StatusNotFound); err != nil {
		return false, err
	}

	return resp.StatusCode == http.StatusOK, nil
}
