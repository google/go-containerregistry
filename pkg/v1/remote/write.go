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
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/google/go-containerregistry/pkg/internal/retry"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"golang.org/x/sync/errgroup"
)

type manifest interface {
	RawManifest() ([]byte, error)
	MediaType() (types.MediaType, error)
	Digest() (v1.Hash, error)
}

// Write pushes the provided img to the specified image reference.
func Write(ref name.Reference, img v1.Image, options ...Option) error {
	log.Println("WRITE-PROFILING Write start")
	ls, err := img.Layers()
	if err != nil {
		return err
	}

	log.Println("WRITE-PROFILING pulled layers")

	o, err := makeOptions(ref.Context().Registry, options...)
	if err != nil {
		return err
	}

	log.Println("WRITE-PROFILING made options")

	scopes := scopesForUploadingImage(ref, ls)
	tr, err := transport.New(ref.Context().Registry, o.auth, o.transport, scopes)
	if err != nil {
		return err
	}
	w, err := newWriter(ref, &http.Client{Transport: tr})
	if err != nil {
		return err
	}
	defer w.Close()

	log.Println("WRITE-PROFILING created writer")

	// Upload individual layers in goroutines and collect any errors.
	// If we can dedupe by the layer digest, try to do so. If we can't determine
	// the digest for whatever reason, we can't dedupe and might re-upload.
	var g errgroup.Group
	uploaded := map[v1.Hash]bool{}

	// precompute digests in parallel
	// computing digests is expensive, and the next stage does it serially
	// since digests are cached, there is no need to store them in an array
	for _, l := range ls {
		l := l

		g.Go(func() error {
			_, err := l.Digest()
			if err != nil {
				return err
			}
			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		return err
	}

	log.Println("WRITE-PROFILING done parallel computing digests")

	g = errgroup.Group{}
	for _, l := range ls {
		l := l

		// Handle foreign layers.
		mt, err := l.MediaType()
		if err != nil {
			return err
		}
		if !mt.IsDistributable() {
			// TODO(jonjohnsonjr): Add "allow-nondistributable-artifacts" option.
			continue
		}

		// Streaming layers calculate their digests while uploading them. Assume
		// an error here indicates we need to upload the layer.
		h, err := l.Digest()
		if err == nil {
			// If we can determine the layer's digest ahead of
			// time, use it to dedupe uploads.
			if uploaded[h] {
				continue // Already uploading.
			}
			uploaded[h] = true
		}

		g.Go(func() error {
			return w.uploadOne(l)
		})
	}

	if l, err := partial.ConfigLayer(img); err != nil {
		// We can't read the ConfigLayer, possibly because of streaming layers,
		// since the layer DiffIDs haven't been calculated yet. Attempt to wait
		// for the other layers to be uploaded, then try the config again.
		if err := g.Wait(); err != nil {
			return err
		}

		// Now that all the layers are uploaded, try to upload the config file blob.
		l, err := partial.ConfigLayer(img)
		if err != nil {
			return err
		}
		if err := w.uploadOne(l); err != nil {
			return err
		}
	} else {
		// We *can* read the ConfigLayer, so upload it concurrently with the layers.
		g.Go(func() error {
			return w.uploadOne(l)
		})

		// Wait for the layers + config.
		if err := g.Wait(); err != nil {
			return err
		}

		log.Println("WRITE-PROFILING done waiting for layers + config")
	}

	// With all of the constituent elements uploaded, upload the manifest
	// to commit the image.
	err = w.commitImage(img)
	log.Println("WRITE-PROFILING committed")
	return err
}

// writer writes the elements of an image to a remote image reference.
type writer struct {
	ref    name.Reference
	client *http.Client

	layerCachePath string      // this is where the cached layers will be stored
	layerCache     cache.Cache // a cache to store compressed layers
}

func newWriter(ref name.Reference, client *http.Client) (*writer, error) {
	cacheDir, err := ioutil.TempDir("", "go-containerregistry-layercache")
	if err != nil {
		return nil, err
	}

	return &writer{
		ref:            ref,
		client:         client,
		layerCachePath: cacheDir,
		layerCache:     cache.NewFilesystemCache(cacheDir),
	}, nil
}

func (w *writer) Close() error {
	return os.RemoveAll(w.layerCachePath)
}

// url returns a url.Url for the specified path in the context of this remote image reference.
func (w *writer) url(path string) url.URL {
	return url.URL{
		Scheme: w.ref.Context().Registry.Scheme(),
		Host:   w.ref.Context().RegistryStr(),
		Path:   path,
	}
}

// nextLocation extracts the fully-qualified URL to which we should send the next request in an upload sequence.
func (w *writer) nextLocation(resp *http.Response) (string, error) {
	loc := resp.Header.Get("Location")
	if len(loc) == 0 {
		return "", errors.New("missing Location header")
	}
	u, err := url.Parse(loc)
	if err != nil {
		return "", err
	}

	// If the location header returned is just a url path, then fully qualify it.
	// We cannot simply call w.url, since there might be an embedded query string.
	return resp.Request.URL.ResolveReference(u).String(), nil
}

// checkExistingBlob checks if a blob exists already in the repository by making a
// HEAD request to the blob store API.  GCR performs an existence check on the
// initiation if "mount" is specified, even if no "from" sources are specified.
// However, this is not broadly applicable to all registries, e.g. ECR.
func (w *writer) checkExistingBlob(h v1.Hash) (bool, error) {
	u := w.url(fmt.Sprintf("/v2/%s/blobs/%s", w.ref.Context().RepositoryStr(), h.String()))

	resp, err := w.client.Head(u.String())
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if err := transport.CheckError(resp, http.StatusOK, http.StatusNotFound); err != nil {
		return false, err
	}

	return resp.StatusCode == http.StatusOK, nil
}

// checkExistingManifest checks if a manifest exists already in the repository
// by making a HEAD request to the manifest API.
func (w *writer) checkExistingManifest(h v1.Hash, mt types.MediaType) (bool, error) {
	u := w.url(fmt.Sprintf("/v2/%s/manifests/%s", w.ref.Context().RepositoryStr(), h.String()))

	req, err := http.NewRequest(http.MethodHead, u.String(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", string(mt))

	resp, err := w.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if err := transport.CheckError(resp, http.StatusOK, http.StatusNotFound); err != nil {
		return false, err
	}

	return resp.StatusCode == http.StatusOK, nil
}

// initiateUpload initiates the blob upload, which starts with a POST that can
// optionally include the hash of the layer and a list of repositories from
// which that layer might be read. On failure, an error is returned.
// On success, the layer was either mounted (nothing more to do) or a blob
// upload was initiated and the body of that blob should be sent to the returned
// location.
func (w *writer) initiateUpload(from, mount string) (location string, mounted bool, err error) {
	u := w.url(fmt.Sprintf("/v2/%s/blobs/uploads/", w.ref.Context().RepositoryStr()))
	uv := url.Values{}
	if mount != "" && from != "" {
		// Quay will fail if we specify a "mount" without a "from".
		uv["mount"] = []string{mount}
		uv["from"] = []string{from}
	}
	u.RawQuery = uv.Encode()

	// Make the request to initiate the blob upload.
	resp, err := w.client.Post(u.String(), "application/json", nil)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if err := transport.CheckError(resp, http.StatusCreated, http.StatusAccepted); err != nil {
		return "", false, err
	}

	// Check the response code to determine the result.
	switch resp.StatusCode {
	case http.StatusCreated:
		// We're done, we were able to fast-path.
		return "", true, nil
	case http.StatusAccepted:
		// Proceed to PATCH, upload has begun.
		loc, err := w.nextLocation(resp)
		return loc, false, err
	default:
		panic("Unreachable: initiateUpload")
	}
}

// streamBlob streams the contents of the blob to the specified location.
// On failure, this will return an error.  On success, this will return the location
// header indicating how to commit the streamed blob.
func (w *writer) streamBlob(blob io.ReadCloser, streamLocation string) (commitLocation string, err error) {
	defer blob.Close()

	req, err := http.NewRequest(http.MethodPatch, streamLocation, blob)
	if err != nil {
		return "", err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := transport.CheckError(resp, http.StatusNoContent, http.StatusAccepted, http.StatusCreated); err != nil {
		return "", err
	}

	// The blob has been uploaded, return the location header indicating
	// how to commit this layer.
	return w.nextLocation(resp)
}

// commitBlob commits this blob by sending a PUT to the location returned from
// streaming the blob.
func (w *writer) commitBlob(location, digest string) error {
	u, err := url.Parse(location)
	if err != nil {
		return err
	}
	v := u.Query()
	v.Set("digest", digest)
	u.RawQuery = v.Encode()

	req, err := http.NewRequest(http.MethodPut, u.String(), nil)
	if err != nil {
		return err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return transport.CheckError(resp, http.StatusCreated)
}

// uploadOne performs a complete upload of a single layer.
func (w *writer) uploadOne(l v1.Layer) error {
	startTime := time.Now()
	var from, mount string
	var digest string
	if h, err := l.Digest(); err == nil {
		digest = h.String()
		log.Printf("UPLOAD-ONE-PROFILING time to get digest for %s: %s\n", digest, time.Since(startTime))

		// If we know the digest, this isn't a streaming layer. Do an existence
		// check so we can skip uploading the layer if possible.
		existing, err := w.checkExistingBlob(h)
		if err != nil {
			return err
		}
		if existing {
			logs.Progress.Printf("existing blob: %v", h)
			return nil
		}

		mount = h.String()

		// cache the compressed layer, so the network op is as fast as possible (some registries lock in weird ways)
		cachedLayer, err := w.layerCache.Put(l)
		if err != nil {
			return err
		}

		// get a compressed reader
		compressedReadCloser, err := cachedLayer.Compressed()
		if err != nil {
			return err
		}
		// as the reader gets read, it'll compress and also write to the file
		_, err = ioutil.ReadAll(compressedReadCloser)
		if err != nil {
			return err
		}

		l, err = w.layerCache.Get(h)
		if err != nil {
			return err
		}
		newDigest, err := l.Digest()
		if err != nil {
			panic(err)
		}
		if newDigest.String() != digest {
			log.Printf("digest changed from %s to %s\n", digest, newDigest.String())
		}
	}
	if ml, ok := l.(*MountableLayer); ok {
		if w.ref.Context().RegistryStr() == ml.Reference.Context().RegistryStr() {
			from = ml.Reference.Context().RepositoryStr()
		}
	}

	tryUpload := func() error {
		location, mounted, err := w.initiateUpload(from, mount)
		if err != nil {
			return err
		} else if mounted {
			h, err := l.Digest()
			if err != nil {
				return err
			}
			logs.Progress.Printf("mounted blob: %s", h.String())
			return nil
		}
		log.Printf("UPLOAD-ONE-PROFILING upload initiated for %s\n", digest)

		blob, err := l.Compressed()
		if err != nil {
			return err
		}
		log.Printf("UPLOAD-ONE-PROFILING compressed for %s\n", digest)
		location, err = w.streamBlob(blob, location)
		if err != nil {
			return err
		}

		log.Printf("UPLOAD-ONE-PROFILING streamed %s\n", digest)

		h, err := l.Digest()
		if err != nil {
			return err
		}

		log.Printf("UPLOAD-ONE-PROFILING again got digest %s\n", digest)
		digest = h.String()

		if err := w.commitBlob(location, digest); err != nil {
			return err
		}
		log.Printf("UPLOAD-ONE-PROFILING committed %s\n", digest)
		logs.Progress.Printf("pushed blob: %s", digest)
		return nil
	}

	// Try this three times, waiting 1s after first failure, 3s after second.
	backoff := retry.Backoff{
		Duration: 1.0 * time.Second,
		Factor:   3.0,
		Jitter:   0.1,
		Steps:    3,
	}

	return retry.Retry(tryUpload, retry.IsTemporary, backoff)
}

// commitImage does a PUT of the image's manifest.
func (w *writer) commitImage(man manifest) error {
	raw, err := man.RawManifest()
	if err != nil {
		return err
	}
	mt, err := man.MediaType()
	if err != nil {
		return err
	}

	u := w.url(fmt.Sprintf("/v2/%s/manifests/%s", w.ref.Context().RepositoryStr(), w.ref.Identifier()))

	// Make the request to PUT the serialized manifest
	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", string(mt))

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := transport.CheckError(resp, http.StatusOK, http.StatusCreated, http.StatusAccepted); err != nil {
		return err
	}

	digest, err := man.Digest()
	if err != nil {
		return err
	}

	// The image was successfully pushed!
	logs.Progress.Printf("%v: digest: %v size: %d", w.ref, digest, len(raw))
	return nil
}

func scopesForUploadingImage(ref name.Reference, layers []v1.Layer) []string {
	// use a map as set to remove duplicates scope strings
	scopeSet := map[string]struct{}{}

	for _, l := range layers {
		if ml, ok := l.(*MountableLayer); ok {
			// we add push scope for ref.Context() after the loop
			if ml.Reference.Context() != ref.Context() {
				scopeSet[ml.Reference.Context().Scope(transport.PullScope)] = struct{}{}
			}
		}
	}

	scopes := make([]string, 0)
	// Push scope should be the first element because a few registries just look at the first scope to determine access.
	scopes = append(scopes, ref.Scope(transport.PushScope))

	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}

	return scopes
}

// WriteIndex pushes the provided ImageIndex to the specified image reference.
// WriteIndex will attempt to push all of the referenced manifests before
// attempting to push the ImageIndex, to retain referential integrity.
func WriteIndex(ref name.Reference, ii v1.ImageIndex, options ...Option) error {
	index, err := ii.IndexManifest()
	if err != nil {
		return err
	}

	o, err := makeOptions(ref.Context().Registry, options...)
	if err != nil {
		return err
	}
	scopes := []string{ref.Scope(transport.PushScope)}
	tr, err := transport.New(ref.Context().Registry, o.auth, o.transport, scopes)
	if err != nil {
		return err
	}
	w, err := newWriter(ref, &http.Client{Transport: tr})
	if err != nil {
		return err
	}
	defer w.Close()

	for _, desc := range index.Manifests {
		ref, err := name.ParseReference(fmt.Sprintf("%s@%s", ref.Context(), desc.Digest), name.StrictValidation)
		if err != nil {
			return err
		}
		exists, err := w.checkExistingManifest(desc.Digest, desc.MediaType)
		if err != nil {
			return err
		}
		if exists {
			logs.Progress.Printf("existing manifest: %v", desc.Digest)
			continue
		}

		switch desc.MediaType {
		case types.OCIImageIndex, types.DockerManifestList:
			ii, err := ii.ImageIndex(desc.Digest)
			if err != nil {
				return err
			}

			if err := WriteIndex(ref, ii, WithAuth(o.auth), WithTransport(o.transport)); err != nil {
				return err
			}
		case types.OCIManifestSchema1, types.DockerManifestSchema2:
			img, err := ii.Image(desc.Digest)
			if err != nil {
				return err
			}
			if err := Write(ref, img, WithAuth(o.auth), WithTransport(o.transport)); err != nil {
				return err
			}
		}
	}

	// With all of the constituent elements uploaded, upload the manifest
	// to commit the image.
	return w.commitImage(ii)
}

// WriteLayer uploads the provided Layer to the specified name.Digest.
func WriteLayer(ref name.Digest, layer v1.Layer, options ...Option) error {
	o, err := makeOptions(ref.Context().Registry, options...)
	if err != nil {
		return err
	}
	scopes := scopesForUploadingImage(ref, []v1.Layer{layer})
	tr, err := transport.New(ref.Context().Registry, o.auth, o.transport, scopes)
	if err != nil {
		return err
	}
	w, err := newWriter(ref, &http.Client{Transport: tr})
	if err != nil {
		return err
	}
	defer w.Close()

	return w.uploadOne(layer)
}
