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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Returns whether this url should be handled by the blob handler
// This is complicated because blob is indicated by the trailing path, not the leading path.
// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#pulling-a-layer
// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#pushing-a-layer
func isBlob(req *http.Request) bool {
	elem := strings.Split(req.URL.Path, "/")
	elem = elem[1:]
	if elem[len(elem)-1] == "" {
		elem = elem[:len(elem)-1]
	}
	if len(elem) < 3 {
		return false
	}
	return elem[len(elem)-2] == "blobs" || (elem[len(elem)-3] == "blobs" &&
		elem[len(elem)-2] == "uploads")
}

// blobs
type blobs struct {
	// Each upload gets a unique id that writes occur to until finalized.
	uploads map[string][]byte
	lock    sync.Mutex

	bh BlobHandler
}

// BlobHandler is the interface for the storage layer underneath this registry.
type BlobHandler interface {
	// Stat returns the size of the blob whose hash is specified, and true,
	// if it exists. If not, it returns (0, false).
	Stat(repo name.Repository, h v1.Hash) (int, bool)

	// Get returns true and a reader for consuming the blob specified with the hash,
	// if it exists.  It now, it returns (nil, false).
	Get(repo name.Repository, h v1.Hash) (io.ReadCloser, bool)

	// Store stores the stream of content with the given hash, or returns the error
	// encountered doing so.
	Store(repo name.Repository, h v1.Hash, content io.ReadCloser) error
}

func (b *blobs) handle(resp http.ResponseWriter, req *http.Request) *regError {
	elem := strings.Split(req.URL.Path, "/")
	elem = elem[1:]
	if elem[len(elem)-1] == "" {
		elem = elem[:len(elem)-1]
	}
	// Must have a path of form /v2/{name}/blobs/{upload,sha256:}
	if len(elem) < 4 {
		return &regError{
			Status:  http.StatusBadRequest,
			Code:    "NAME_INVALID",
			Message: "blobs must be attached to a repo",
		}
	}
	target := elem[len(elem)-1]
	service := elem[len(elem)-2]
	digest := req.URL.Query().Get("digest")
	contentRange := req.Header.Get("Content-Range")

	switch req.Method {
	case http.MethodHead:
		h, err := v1.NewHash(target)
		if err != nil {
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "NAME_INVALID",
				Message: err.Error(),
			}
		}
		repo, err := name.NewRepository(req.URL.Host + path.Join(elem[1:len(elem)-2]...))
		if err != nil {
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "NAME_INVALID",
				Message: err.Error(),
			}
		}

		sz, ok := b.bh.Stat(repo, h)
		if !ok {
			return &regError{
				Status:  http.StatusNotFound,
				Code:    "BLOB_UNKNOWN",
				Message: "Unknown blob",
			}
		}

		resp.Header().Set("Content-Length", fmt.Sprint(sz))
		resp.Header().Set("Docker-Content-Digest", target)
		resp.WriteHeader(http.StatusOK)
		return nil

	case http.MethodGet:
		h, err := v1.NewHash(target)
		if err != nil {
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "NAME_INVALID",
				Message: err.Error(),
			}
		}
		repo, err := name.NewRepository(req.URL.Host + path.Join(elem[1:len(elem)-2]...))
		if err != nil {
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "NAME_INVALID",
				Message: err.Error(),
			}
		}

		sz, ok := b.bh.Stat(repo, h)
		if !ok {
			return &regError{
				Status:  http.StatusNotFound,
				Code:    "BLOB_UNKNOWN",
				Message: "Unknown blob",
			}
		}

		b, ok := b.bh.Get(repo, h)
		if !ok {
			return &regError{
				Status:  http.StatusNotFound,
				Code:    "BLOB_UNKNOWN",
				Message: "Unknown blob",
			}
		}
		defer b.Close()

		resp.Header().Set("Content-Length", fmt.Sprint(sz))
		resp.Header().Set("Docker-Content-Digest", target)
		resp.WriteHeader(http.StatusOK)
		io.Copy(resp, b)
		return nil

	case http.MethodPost:
		// It is weird that this is "target" instead of "service", but
		// that's how the index math works out above.
		if target != "uploads" {
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "METHOD_UNKNOWN",
				Message: fmt.Sprintf("POST to /blobs must be followed by /uploads, got %s", target),
			}
		}

		if digest != "" {
			l := &bytes.Buffer{}
			io.Copy(l, req.Body)
			rd := sha256.Sum256(l.Bytes())
			d := "sha256:" + hex.EncodeToString(rd[:])
			if d != digest {
				return &regError{
					Status:  http.StatusBadRequest,
					Code:    "DIGEST_INVALID",
					Message: "digest does not match contents",
				}
			}
			h, err := v1.NewHash(d)
			if err != nil {
				// This is not reachable
				return &regError{
					Status:  http.StatusBadRequest,
					Code:    "NAME_INVALID",
					Message: err.Error(),
				}
			}
			repo, err := name.NewRepository(req.URL.Host + path.Join(elem[1:len(elem)-2]...))
			if err != nil {
				return &regError{
					Status:  http.StatusBadRequest,
					Code:    "NAME_INVALID",
					Message: err.Error(),
				}
			}

			if err := b.bh.Store(repo, h, ioutil.NopCloser(l)); err != nil {
				return &regError{
					Status:  http.StatusInternalServerError,
					Code:    "BLOB_UPLOAD_INVALID",
					Message: err.Error(),
				}
			}
			resp.Header().Set("Docker-Content-Digest", d)
			resp.WriteHeader(http.StatusCreated)
			return nil
		}

		id := fmt.Sprint(rand.Int63())
		resp.Header().Set("Location", "/"+path.Join("v2", path.Join(elem[1:len(elem)-2]...), "blobs/uploads", id))
		resp.Header().Set("Range", "0-0")
		resp.WriteHeader(http.StatusAccepted)
		return nil

	case http.MethodPatch:
		if service != "uploads" {
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "METHOD_UNKNOWN",
				Message: fmt.Sprintf("PATCH to /blobs must be followed by /uploads, got %s", service),
			}
		}

		if contentRange != "" {
			start, end := 0, 0
			if _, err := fmt.Sscanf(contentRange, "%d-%d", &start, &end); err != nil {
				return &regError{
					Status:  http.StatusRequestedRangeNotSatisfiable,
					Code:    "BLOB_UPLOAD_UNKNOWN",
					Message: "We don't understand your Content-Range",
				}
			}
			b.lock.Lock()
			defer b.lock.Unlock()
			if start != len(b.uploads[target]) {
				return &regError{
					Status:  http.StatusRequestedRangeNotSatisfiable,
					Code:    "BLOB_UPLOAD_UNKNOWN",
					Message: "Your content range doesn't match what we have",
				}
			}
			l := bytes.NewBuffer(b.uploads[target])
			io.Copy(l, req.Body)
			b.uploads[target] = l.Bytes()
			resp.Header().Set("Location", "/"+path.Join("v2", path.Join(elem[1:len(elem)-3]...), "blobs/uploads", target))
			resp.Header().Set("Range", fmt.Sprintf("0-%d", len(l.Bytes())-1))
			resp.WriteHeader(http.StatusNoContent)
			return nil
		}

		b.lock.Lock()
		defer b.lock.Unlock()
		if _, ok := b.uploads[target]; ok {
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "BLOB_UPLOAD_INVALID",
				Message: "Stream uploads after first write are not allowed",
			}
		}

		l := &bytes.Buffer{}
		io.Copy(l, req.Body)

		b.uploads[target] = l.Bytes()
		resp.Header().Set("Location", "/"+path.Join("v2", path.Join(elem[1:len(elem)-3]...), "blobs/uploads", target))
		resp.Header().Set("Range", fmt.Sprintf("0-%d", len(l.Bytes())-1))
		resp.WriteHeader(http.StatusNoContent)
		return nil

	case http.MethodPut:
		if service != "uploads" {
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "METHOD_UNKNOWN",
				Message: fmt.Sprintf("PUT to /blobs must be followed by /uploads, got %s", service),
			}
		}

		if digest == "" {
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "DIGEST_INVALID",
				Message: "digest not specified",
			}
		}

		b.lock.Lock()
		defer b.lock.Unlock()
		l := bytes.NewBuffer(b.uploads[target])
		io.Copy(l, req.Body)
		rd := sha256.Sum256(l.Bytes())
		d := "sha256:" + hex.EncodeToString(rd[:])
		if d != digest {
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "DIGEST_INVALID",
				Message: "digest does not match contents",
			}
		}
		repo, err := name.NewRepository(req.URL.Host + path.Join(elem[1:len(elem)-3]...))
		if err != nil {
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "NAME_INVALID",
				Message: err.Error(),
			}
		}
		h, err := v1.NewHash(digest)
		if err != nil {
			// This is not reachable
			return &regError{
				Status:  http.StatusBadRequest,
				Code:    "NAME_INVALID",
				Message: err.Error(),
			}
		}

		if err := b.bh.Store(repo, h, ioutil.NopCloser(l)); err != nil {
			return &regError{
				Status:  http.StatusInternalServerError,
				Code:    "BLOB_UPLOAD_INVALID",
				Message: err.Error(),
			}
		}

		delete(b.uploads, target)
		resp.Header().Set("Docker-Content-Digest", d)
		resp.WriteHeader(http.StatusCreated)
		return nil

	default:
		return &regError{
			Status:  http.StatusBadRequest,
			Code:    "METHOD_UNKNOWN",
			Message: "We don't understand your method + url",
		}
	}
}

type defaultBlobStore struct {
	m        sync.Mutex
	contents map[v1.Hash][]byte
}

var _ BlobHandler = (*defaultBlobStore)(nil)

// Stat implements BlobHandler
func (dbs *defaultBlobStore) Stat(repo name.Repository, h v1.Hash) (int, bool) {
	dbs.m.Lock()
	defer dbs.m.Unlock()
	b, ok := dbs.contents[h]
	if !ok {
		return 0, false
	}
	return len(b), true
}

// Get implements BlobHandler
func (dbs *defaultBlobStore) Get(repo name.Repository, h v1.Hash) (io.ReadCloser, bool) {
	dbs.m.Lock()
	defer dbs.m.Unlock()
	b, ok := dbs.contents[h]
	return ioutil.NopCloser(bytes.NewBuffer(b)), ok
}

// Store implements BlobHandler
func (dbs *defaultBlobStore) Store(repo name.Repository, h v1.Hash, rc io.ReadCloser) error {
	dbs.m.Lock()
	defer dbs.m.Unlock()
	defer rc.Close()
	b, err := ioutil.ReadAll(rc)
	if err != nil {
		return err
	}
	dbs.contents[h] = b
	return nil
}
