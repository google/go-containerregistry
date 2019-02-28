package remote

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// remoteIndex accesses an index from a remote registry
type remoteIndex struct {
	ref          name.Reference
	client       *http.Client
	manifestLock sync.Mutex // Protects manifest
	manifest     []byte
	mediaType    types.MediaType
}

// Index provides access to a remote index reference, applying functional options
// to the underlying imageOpener before resolving the reference into a v1.ImageIndex.
func Index(ref name.Reference, options ...ImageOption) (v1.ImageIndex, error) {
	i := &imageOpener{
		auth:      authn.Anonymous,
		transport: http.DefaultTransport,
		ref:       ref,
	}

	for _, option := range options {
		if err := option(i); err != nil {
			return nil, err
		}
	}
	tr, err := transport.New(i.ref.Context().Registry, i.auth, i.transport, []string{i.ref.Scope(transport.PullScope)})
	if err != nil {
		return nil, err
	}
	return &remoteIndex{
		ref:    i.ref,
		client: &http.Client{Transport: tr},
	}, nil
}

func (r *remoteIndex) url(resource, identifier string) url.URL {
	return url.URL{
		Scheme: r.ref.Context().Registry.Scheme(),
		Host:   r.ref.Context().RegistryStr(),
		Path:   fmt.Sprintf("/v2/%s/%s/%s", r.ref.Context().RepositoryStr(), resource, identifier),
	}
}

func (r *remoteIndex) MediaType() (types.MediaType, error) {
	if string(r.mediaType) != "" {
		return r.mediaType, nil
	}
	return types.DockerManifestList, nil
}

func (r *remoteIndex) Digest() (v1.Hash, error) {
	return partial.Digest(r)
}

func (r *remoteIndex) RawManifest() ([]byte, error) {
	r.manifestLock.Lock()
	defer r.manifestLock.Unlock()
	if r.manifest != nil {
		return r.manifest, nil
	}

	u := r.url("manifests", r.ref.Identifier())
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", strings.Join([]string{
		string(types.DockerManifestList),
		string(types.OCIImageIndex),
	}, ","))
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := transport.CheckError(resp, http.StatusOK); err != nil {
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
	} else {
		// Do nothing for tags; I give up.
		//
		// We'd like to validate that the "Docker-Content-Digest" header matches what is returned by the registry,
		// but so many registries implement this incorrectly that it's not worth checking.
		//
		// For reference:
		// https://github.com/docker/distribution/issues/2395
		// https://github.com/GoogleContainerTools/kaniko/issues/298
	}

	r.mediaType = types.MediaType(resp.Header.Get("Content-Type"))
	r.manifest = manifest
	return r.manifest, nil
}

func (r *remoteIndex) IndexManifest() (*v1.IndexManifest, error) {
	b, err := r.RawManifest()
	if err != nil {
		return nil, err
	}
	return v1.ParseIndexManifest(bytes.NewReader(b))
}

func (r *remoteIndex) Image(h v1.Hash) (v1.Image, error) {
	imgRef, err := name.ParseReference(fmt.Sprintf("%s@%s", r.ref.Context(), h), name.StrictValidation)
	if err != nil {
		return nil, err
	}
	// TODO: pull this out for reuse
	ri := &remoteImage{
		ref:    imgRef,
		client: r.client,
	}
	imgCore, err := partial.CompressedToImage(ri)
	if err != nil {
		return imgCore, err
	}
	// Wrap the v1.Layers returned by this v1.Image in a hint for downstream
	// remote.Write calls to facilitate cross-repo "mounting".
	return &mountableImage{
		Image:     imgCore,
		Reference: r.ref,
	}, nil
}

func (r *remoteIndex) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
	idxRef, err := name.ParseReference(fmt.Sprintf("%s@%s", r.ref.Context(), h), name.StrictValidation)
	if err != nil {
		return nil, err
	}
	// TODO: pull this out for reuse
	return &remoteIndex{
		ref:    idxRef,
		client: r.client,
	}, nil
}
