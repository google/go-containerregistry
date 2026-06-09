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

package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type catalog struct {
	Repos []string `json:"repositories"`
}

type listTags struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// Manifest is the stored representation of a manifest: its raw bytes plus the
// content type it was uploaded with. It is exported so that out-of-package
// ManifestHandler implementations can construct and return values.
type Manifest struct {
	ContentType string
	Blob        []byte
}

// ErrNameUnknown signals that the repository itself is unknown, as opposed to a
// known repository that is missing the requested manifest (ErrNotFound). It
// lets ManifestHandler implementations preserve the registry's distinction
// between the NAME_UNKNOWN and MANIFEST_UNKNOWN errors. Handlers that only
// return ErrNotFound degrade gracefully to MANIFEST_UNKNOWN.
var ErrNameUnknown = errors.New("name unknown")

// ManifestHandler represents a minimal manifest storage backend, capable of
// getting, putting, and deleting manifests by tag or digest.
type ManifestHandler interface {
	// Get returns the manifest stored under repo for the given reference (a tag
	// or a "sha256:..." digest string), ErrNameUnknown if the repository is
	// unknown, or ErrNotFound if the reference is unknown.
	Get(ctx context.Context, repo, target string) (*Manifest, error)

	// Put stores a manifest under repo. The registry computes the digest and
	// passes it alongside the reference (target) the client used. The manifest
	// must be made retrievable by BOTH the digest string and the target string.
	Put(ctx context.Context, repo, target, digest string, manifest *Manifest) error

	// Delete removes the manifest stored under repo for the given reference,
	// returning ErrNameUnknown or ErrNotFound as Get does.
	Delete(ctx context.Context, repo, target string) error
}

// ManifestTagLister is an extension interface for backends that can enumerate
// the tags in a repository. It is required for the tags-list endpoint.
type ManifestTagLister interface {
	// ListTags returns the tag references in repo (excluding digest
	// references), or ErrNameUnknown if the repository is unknown.
	ListTags(ctx context.Context, repo string) ([]string, error)
}

// ManifestCataloger is an extension interface for backends that can enumerate
// repositories. It is required for the _catalog endpoint.
type ManifestCataloger interface {
	// ListRepos returns all repository names known to the backend.
	ListRepos(ctx context.Context) ([]string, error)
}

// ManifestReferrerLister is an extension interface for backends that can
// enumerate all manifests in a repository by digest. It is required for the
// referrers API.
type ManifestReferrerLister interface {
	// ListDigests returns digest -> Manifest for every digest-keyed manifest
	// stored under repo, or ErrNameUnknown if the repository is unknown.
	ListDigests(ctx context.Context, repo string) (map[string]Manifest, error)
}

type memManifestHandler struct {
	// maps repo -> manifest tag/digest -> manifest
	manifests map[string]map[string]Manifest
	lock      sync.RWMutex
}

// NewInMemoryManifestHandler returns a ManifestHandler that stores manifests in
// memory. It is the default backend used by New.
func NewInMemoryManifestHandler() ManifestHandler {
	return &memManifestHandler{manifests: map[string]map[string]Manifest{}}
}

// Compile-time assertions that the in-memory handler satisfies every extension.
var (
	_ ManifestTagLister      = (*memManifestHandler)(nil)
	_ ManifestCataloger      = (*memManifestHandler)(nil)
	_ ManifestReferrerLister = (*memManifestHandler)(nil)
)

func (m *memManifestHandler) Get(_ context.Context, repo, target string) (*Manifest, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	c, ok := m.manifests[repo]
	if !ok {
		return nil, ErrNameUnknown
	}
	mf, ok := c[target]
	if !ok {
		return nil, ErrNotFound
	}
	return &mf, nil
}

func (m *memManifestHandler) Put(_ context.Context, repo, target, digest string, manifest *Manifest) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.manifests[repo]; !ok {
		m.manifests[repo] = make(map[string]Manifest, 2)
	}

	// Allow future references by target (tag) and immutable digest.
	// See https://docs.docker.com/engine/reference/commandline/pull/#pull-an-image-by-digest-immutable-identifier.
	m.manifests[repo][digest] = *manifest
	m.manifests[repo][target] = *manifest
	return nil
}

func (m *memManifestHandler) Delete(_ context.Context, repo, target string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.manifests[repo]; !ok {
		return ErrNameUnknown
	}
	if _, ok := m.manifests[repo][target]; !ok {
		return ErrNotFound
	}
	delete(m.manifests[repo], target)
	return nil
}

func (m *memManifestHandler) ListTags(_ context.Context, repo string) ([]string, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	c, ok := m.manifests[repo]
	if !ok {
		return nil, ErrNameUnknown
	}
	var tags []string
	for tag := range c {
		if !strings.Contains(tag, "sha256:") {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func (m *memManifestHandler) ListRepos(_ context.Context) ([]string, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	repos := make([]string, 0, len(m.manifests))
	for key := range m.manifests {
		repos = append(repos, key)
	}
	return repos, nil
}

func (m *memManifestHandler) ListDigests(_ context.Context, repo string) (map[string]Manifest, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	c, ok := m.manifests[repo]
	if !ok {
		return nil, ErrNameUnknown
	}
	out := make(map[string]Manifest, len(c))
	for ref, mf := range c {
		if _, err := v1.NewHash(ref); err == nil {
			out[ref] = mf
		}
	}
	return out, nil
}

type manifests struct {
	manifestHandler ManifestHandler
	log             *log.Logger
}

func isManifest(req *http.Request) bool {
	elems := strings.Split(req.URL.Path, "/")
	elems = elems[1:]
	if len(elems) < 4 {
		return false
	}
	return elems[len(elems)-2] == "manifests"
}

func isTags(req *http.Request) bool {
	elems := strings.Split(req.URL.Path, "/")
	elems = elems[1:]
	if len(elems) < 4 {
		return false
	}
	return elems[len(elems)-2] == "tags"
}

func isCatalog(req *http.Request) bool {
	elems := strings.Split(req.URL.Path, "/")
	elems = elems[1:]
	if len(elems) < 2 {
		return false
	}

	return elems[len(elems)-1] == "_catalog"
}

// Returns whether this url should be handled by the referrers handler
func isReferrers(req *http.Request) bool {
	elems := strings.Split(req.URL.Path, "/")
	elems = elems[1:]
	if len(elems) < 4 {
		return false
	}
	return elems[len(elems)-2] == "referrers"
}

// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#pulling-an-image-manifest
// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#pushing-an-image
func (m *manifests) handle(resp http.ResponseWriter, req *http.Request) *regError {
	elem := strings.Split(req.URL.Path, "/")
	elem = elem[1:]
	target := elem[len(elem)-1]
	repo := strings.Join(elem[1:len(elem)-2], "/")

	switch req.Method {
	case http.MethodGet, http.MethodHead:
		mf, err := m.manifestHandler.Get(req.Context(), repo, target)
		if errors.Is(err, ErrNameUnknown) {
			return &regError{
				Status:  http.StatusNotFound,
				Code:    "NAME_UNKNOWN",
				Message: "Unknown name",
			}
		} else if errors.Is(err, ErrNotFound) {
			return &regError{
				Status:  http.StatusNotFound,
				Code:    "MANIFEST_UNKNOWN",
				Message: "Unknown manifest",
			}
		} else if err != nil {
			return regErrInternal(err)
		}

		h, _, _ := v1.SHA256(bytes.NewReader(mf.Blob))
		resp.Header().Set("Docker-Content-Digest", h.String())
		resp.Header().Set("Content-Type", mf.ContentType)
		resp.Header().Set("Content-Length", fmt.Sprint(len(mf.Blob)))
		resp.WriteHeader(http.StatusOK)
		if req.Method == http.MethodGet {
			io.Copy(resp, bytes.NewReader(mf.Blob))
		}
		return nil

	case http.MethodPut:
		b := &bytes.Buffer{}
		io.Copy(b, req.Body)
		h, _, _ := v1.SHA256(bytes.NewReader(b.Bytes()))
		digest := h.String()
		mf := &Manifest{
			Blob:        b.Bytes(),
			ContentType: req.Header.Get("Content-Type"),
		}

		// If the manifest is a manifest list, check that the manifest
		// list's constituent manifests are already uploaded.
		// This isn't strictly required by the registry API, but some
		// registries require this.
		if types.MediaType(mf.ContentType).IsIndex() {
			im, err := v1.ParseIndexManifest(bytes.NewReader(b.Bytes()))
			if err != nil {
				return &regError{
					Status:  http.StatusBadRequest,
					Code:    "MANIFEST_INVALID",
					Message: err.Error(),
				}
			}
			for _, desc := range im.Manifests {
				if !desc.MediaType.IsDistributable() {
					continue
				}
				if desc.MediaType.IsIndex() || desc.MediaType.IsImage() {
					if _, err := m.manifestHandler.Get(req.Context(), repo, desc.Digest.String()); errors.Is(err, ErrNameUnknown) || errors.Is(err, ErrNotFound) {
						return &regError{
							Status:  http.StatusNotFound,
							Code:    "MANIFEST_UNKNOWN",
							Message: fmt.Sprintf("Sub-manifest %q not found", desc.Digest),
						}
					} else if err != nil {
						return regErrInternal(err)
					}
				} else {
					// TODO: Probably want to do an existence check for blobs.
					m.log.Printf("TODO: Check blobs for %q", desc.Digest)
				}
			}
		}

		if err := m.manifestHandler.Put(req.Context(), repo, target, digest, mf); err != nil {
			return regErrInternal(err)
		}
		resp.Header().Set("Docker-Content-Digest", digest)
		resp.WriteHeader(http.StatusCreated)
		return nil

	case http.MethodDelete:
		err := m.manifestHandler.Delete(req.Context(), repo, target)
		if errors.Is(err, ErrNameUnknown) {
			return &regError{
				Status:  http.StatusNotFound,
				Code:    "NAME_UNKNOWN",
				Message: "Unknown name",
			}
		} else if errors.Is(err, ErrNotFound) {
			return &regError{
				Status:  http.StatusNotFound,
				Code:    "MANIFEST_UNKNOWN",
				Message: "Unknown manifest",
			}
		} else if err != nil {
			return regErrInternal(err)
		}

		resp.WriteHeader(http.StatusAccepted)
		return nil

	default:
		return &regError{
			Status:  http.StatusBadRequest,
			Code:    "METHOD_UNKNOWN",
			Message: "We don't understand your method + url",
		}
	}
}

func (m *manifests) handleTags(resp http.ResponseWriter, req *http.Request) *regError {
	elem := strings.Split(req.URL.Path, "/")
	elem = elem[1:]
	repo := strings.Join(elem[1:len(elem)-2], "/")

	if req.Method == "GET" {
		tl, ok := m.manifestHandler.(ManifestTagLister)
		if !ok {
			return regErrUnsupported
		}

		tags, err := tl.ListTags(req.Context(), repo)
		if errors.Is(err, ErrNameUnknown) {
			return &regError{
				Status:  http.StatusNotFound,
				Code:    "NAME_UNKNOWN",
				Message: "Unknown name",
			}
		} else if err != nil {
			return regErrInternal(err)
		}
		sort.Strings(tags)

		// https://github.com/opencontainers/distribution-spec/blob/b505e9cc53ec499edbd9c1be32298388921bb705/detail.md#tags-paginated
		// Offset using last query parameter.
		if last := req.URL.Query().Get("last"); last != "" {
			for i, t := range tags {
				if t > last {
					tags = tags[i:]
					break
				}
			}
		}

		// Limit using n query parameter.
		if ns := req.URL.Query().Get("n"); ns != "" {
			if n, err := strconv.Atoi(ns); err != nil {
				return &regError{
					Status:  http.StatusBadRequest,
					Code:    "BAD_REQUEST",
					Message: fmt.Sprintf("parsing n: %v", err),
				}
			} else if n < len(tags) {
				tags = tags[:n]
			}
		}

		tagsToList := listTags{
			Name: repo,
			Tags: tags,
		}

		msg, _ := json.Marshal(tagsToList)
		resp.Header().Set("Content-Length", fmt.Sprint(len(msg)))
		resp.WriteHeader(http.StatusOK)
		io.Copy(resp, bytes.NewReader([]byte(msg)))
		return nil
	}

	return &regError{
		Status:  http.StatusBadRequest,
		Code:    "METHOD_UNKNOWN",
		Message: "We don't understand your method + url",
	}
}

func (m *manifests) handleCatalog(resp http.ResponseWriter, req *http.Request) *regError {
	query := req.URL.Query()
	nStr := query.Get("n")
	n := 10000
	if nStr != "" {
		n, _ = strconv.Atoi(nStr)
	}

	if req.Method == "GET" {
		cat, ok := m.manifestHandler.(ManifestCataloger)
		if !ok {
			return regErrUnsupported
		}

		allRepos, err := cat.ListRepos(req.Context())
		if err != nil {
			return regErrInternal(err)
		}

		var repos []string
		countRepos := 0
		// TODO: implement pagination
		for _, key := range allRepos {
			if countRepos >= n {
				break
			}
			countRepos++

			repos = append(repos, key)
		}

		repositoriesToList := catalog{
			Repos: repos,
		}

		msg, _ := json.Marshal(repositoriesToList)
		resp.Header().Set("Content-Length", fmt.Sprint(len(msg)))
		resp.WriteHeader(http.StatusOK)
		io.Copy(resp, bytes.NewReader([]byte(msg)))
		return nil
	}

	return &regError{
		Status:  http.StatusBadRequest,
		Code:    "METHOD_UNKNOWN",
		Message: "We don't understand your method + url",
	}
}

// TODO: implement handling of artifactType querystring
func (m *manifests) handleReferrers(resp http.ResponseWriter, req *http.Request) *regError {
	// Ensure this is a GET request
	if req.Method != "GET" {
		return &regError{
			Status:  http.StatusBadRequest,
			Code:    "METHOD_UNKNOWN",
			Message: "We don't understand your method + url",
		}
	}

	elem := strings.Split(req.URL.Path, "/")
	elem = elem[1:]
	target := elem[len(elem)-1]
	repo := strings.Join(elem[1:len(elem)-2], "/")

	// Validate that incoming target is a valid digest
	if _, err := v1.NewHash(target); err != nil {
		return &regError{
			Status:  http.StatusBadRequest,
			Code:    "UNSUPPORTED",
			Message: "Target must be a valid digest",
		}
	}

	rl, ok := m.manifestHandler.(ManifestReferrerLister)
	if !ok {
		return regErrUnsupported
	}

	digestToManifestMap, err := rl.ListDigests(req.Context(), repo)
	if errors.Is(err, ErrNameUnknown) {
		return &regError{
			Status:  http.StatusNotFound,
			Code:    "NAME_UNKNOWN",
			Message: "Unknown name",
		}
	} else if err != nil {
		return regErrInternal(err)
	}

	im := v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     types.OCIImageIndex,
		Manifests:     []v1.Descriptor{},
	}
	for digest, manifest := range digestToManifestMap {
		h, err := v1.NewHash(digest)
		if err != nil {
			continue
		}
		var refPointer struct {
			Subject *v1.Descriptor `json:"subject"`
		}
		json.Unmarshal(manifest.Blob, &refPointer)
		if refPointer.Subject == nil {
			continue
		}
		referenceDigest := refPointer.Subject.Digest
		if referenceDigest.String() != target {
			continue
		}
		// At this point, we know the current digest references the target
		var imageAsArtifact struct {
			Config struct {
				MediaType string `json:"mediaType"`
			} `json:"config"`
		}
		json.Unmarshal(manifest.Blob, &imageAsArtifact)
		im.Manifests = append(im.Manifests, v1.Descriptor{
			MediaType:    types.MediaType(manifest.ContentType),
			Size:         int64(len(manifest.Blob)),
			Digest:       h,
			ArtifactType: imageAsArtifact.Config.MediaType,
		})
	}
	msg, _ := json.Marshal(&im)
	resp.Header().Set("Content-Length", fmt.Sprint(len(msg)))
	resp.Header().Set("Content-Type", string(types.OCIImageIndex))
	resp.WriteHeader(http.StatusOK)
	io.Copy(resp, bytes.NewReader([]byte(msg)))
	return nil
}
