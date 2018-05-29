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

package remote

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"

	"github.com/google/go-containerregistry/authn"
	"github.com/google/go-containerregistry/name"
	"github.com/google/go-containerregistry/v1"
	"github.com/google/go-containerregistry/v1/partial"
	"github.com/google/go-containerregistry/v1/remote/transport"
	"github.com/google/go-containerregistry/v1/types"
	"github.com/google/go-containerregistry/v1/v1util"
)

// remoteImage accesses an image from a remote registry
type remoteImage struct {
	ref  name.Reference
	auth authn.Authenticator
	t    http.RoundTripper

	client       *http.Client
	cache        Cache
	manifestLock sync.Mutex // Protects manifest
	manifest     []byte
	configLock   sync.Mutex // Protects config
	config       []byte

	initOnce sync.Once
}

var _ partial.CompressedImageCore = (*remoteImage)(nil)

type ImageOptions struct {
	Cache Cache
}

func (o *ImageOptions) cache() Cache {
	if o == nil {
		return nil
	}
	return o.Cache
}

// ErrCacheMiss is the error returned by implementations of Cache.Load when the
// blob was not found in the cache.
var ErrCacheMiss = errors.New("blob not found in cache")

type Cache interface {
	Load(v1.Hash) (io.ReadCloser, error)
	Store(v1.Hash, io.Reader) error
}

// Image accesses a given image reference over the provided transport, with the provided authentication.
func Image(ref name.Reference, auth authn.Authenticator, t http.RoundTripper, opt *ImageOptions) (v1.Image, error) {
	return partial.CompressedToImage(&remoteImage{
		ref:   ref,
		auth:  auth,
		t:     t,
		cache: opt.cache(),
	})
}

func (r *remoteImage) url(resource, identifier string) url.URL {
	return url.URL{
		Scheme: transport.Scheme(r.ref.Context().Registry),
		Host:   r.ref.Context().RegistryStr(),
		Path:   fmt.Sprintf("/v2/%s/%s/%s", r.ref.Context().RepositoryStr(), resource, identifier),
	}
}

func (r *remoteImage) MediaType() (types.MediaType, error) {
	// TODO(jonjohnsonjr): Determine this based on response.
	return types.DockerManifestSchema2, nil
}

// TODO(jonjohnsonjr): Handle manifest lists.
func (r *remoteImage) RawManifest() ([]byte, error) {
	r.manifestLock.Lock()
	defer r.manifestLock.Unlock()
	if r.manifest != nil {
		return r.manifest, nil
	}

	// If the image is specified by digest and a cache implementation is
	// provided, attempt to lookup in the cache.
	if dig, ok := r.ref.(name.Digest); ok && r.cache != nil {
		h, err := v1.NewHash(dig.DigestStr())
		if err != nil {
			return nil, err
		}
		rc, err := r.cache.Load(h)
		if err != nil && err != ErrCacheMiss {
			return nil, err
		}
		defer rc.Close()
		r.manifest, err = ioutil.ReadAll(rc)
		return r.manifest, err
	}

	// Initialize the HTTP transport, if this is the first time its needed.
	var initErr error
	r.initOnce.Do(func() {
		scopes := []string{r.ref.Scope(transport.PullScope)}
		tr, err := transport.New(r.ref.Context().Registry, r.auth, r.t, scopes)
		if err != nil {
			initErr = err
			return
		}
		r.client = &http.Client{Transport: tr}
	})
	if initErr != nil {
		return nil, initErr
	}

	u := r.url("manifests", r.ref.Identifier())
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	// TODO(jonjohnsonjr): Accept OCI manifest, manifest list, and image index.
	req.Header.Set("Accept", string(types.DockerManifestSchema2))
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkError(resp, http.StatusOK); err != nil {
		return nil, err
	}

	manifest, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	digest, _, err := v1.SHA256(bytes.NewReader(manifest))
	if err != nil {
		return nil, err
	}

	// Validate the digest matches what we asked for, if pulling by digest.
	if dgst, ok := r.ref.(name.Digest); ok {
		if digest.String() != dgst.DigestStr() {
			return nil, fmt.Errorf("manifest digest: %q does not match requested digest: %q for %q", digest, dgst.DigestStr(), r.ref)
		}
	} else if checksum := resp.Header.Get("Docker-Content-Digest"); checksum != "" && checksum != digest.String() {
		err := fmt.Errorf("manifest digest: %q does not match Docker-Content-Digest: %q for %q", digest, checksum, r.ref)
		if r.ref.Context().RegistryStr() == name.DefaultRegistry {
			// TODO(docker/distribution#2395): Remove this check.
		} else {
			// When pulling by tag, we can only validate that the digest matches what the registry told us it should be.
			return nil, err
		}
	}

	// If a cache implementation is provided, attempt to store the blob in
	// the cache.
	if r.cache != nil {
		if err := r.cache.Store(digest, ioutil.NopCloser(bytes.NewReader(manifest))); err != nil {
			return nil, err
		}
	}

	r.manifest = manifest
	return r.manifest, nil
}

func (r *remoteImage) RawConfigFile() ([]byte, error) {
	r.configLock.Lock()
	defer r.configLock.Unlock()
	if r.config != nil {
		return r.config, nil
	}

	m, err := partial.Manifest(r)
	if err != nil {
		return nil, err
	}

	cl, err := r.LayerByDigest(m.Config.Digest)
	if err != nil {
		return nil, err
	}
	body, err := cl.Compressed()
	if err != nil {
		return nil, err
	}
	defer body.Close()

	r.config, err = ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return r.config, nil
}

// remoteLayer implements partial.CompressedLayer
type remoteLayer struct {
	ri     *remoteImage
	digest v1.Hash
}

// Digest implements partial.CompressedLayer
func (rl *remoteLayer) Digest() (v1.Hash, error) {
	return rl.digest, nil
}

// Compressed implements partial.CompressedLayer
func (rl *remoteLayer) Compressed() (io.ReadCloser, error) {
	if rl.ri.cache != nil {
		if rc, err := rl.ri.cache.Load(rl.digest); err != nil && err != ErrCacheMiss {
			return nil, err
		} else if err == nil {
			return rc, nil
		}
	}

	u := rl.ri.url("blobs", rl.digest.String())
	resp, err := rl.ri.client.Get(u.String())
	if err != nil {
		return nil, err
	}

	if err := checkError(resp, http.StatusOK); err != nil {
		resp.Body.Close()
		return nil, err
	}

	// If a cache implementation is provided, attempt to store the blob in
	// the cache.
	if rl.ri.cache != nil {
		defer resp.Body.Close()
		if err := rl.ri.cache.Store(rl.digest, resp.Body); err != nil {
			return nil, err
		}
		return rl.ri.cache.Load(rl.digest)
	}

	return v1util.VerifyReadCloser(resp.Body, rl.digest)
}

// Manifest implements partial.WithManifest so that we can use partial.BlobSize below.
func (rl *remoteLayer) Manifest() (*v1.Manifest, error) {
	return partial.Manifest(rl.ri)
}

// Size implements partial.CompressedLayer
func (rl *remoteLayer) Size() (int64, error) {
	// Look up the size of this digest in the manifest to avoid a request.
	return partial.BlobSize(rl, rl.digest)
}

// LayerByDigest implements partial.CompressedLayer
func (r *remoteImage) LayerByDigest(h v1.Hash) (partial.CompressedLayer, error) {
	return &remoteLayer{
		ri:     r,
		digest: h,
	}, nil
}
