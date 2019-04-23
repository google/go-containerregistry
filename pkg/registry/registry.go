// Package registry implements a docker V2 registry and the OCI distribution specification.
//
// It is designed to be used anywhere a low dependency container registry is needed, with an
// initial focus on tests.
//
// Its goal is to be standards compliant and its strictness will increase over time.

package registry

import (
	"net/http"
)

type v struct {
	blobs     blobs
	manifests manifests
}

// https://docs.docker.com/registry/spec/api/#api-version-check
// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#api-version-check
func (v *v) v2(resp http.ResponseWriter, req *http.Request) {
	if isBlob(req) {
		v.blobs.handle(resp, req)
		return
	}
	if isManifest(req) {
		v.manifests.handle(resp, req)
		return
	}
	resp.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	if req.URL.Path != "/v2/" {
		resp.WriteHeader(404)
		return
	}
	resp.WriteHeader(200)
}

// New returns a handler which implements the docker registry protocol. It should be registered at the site root.
func New() http.Handler {
	m := http.NewServeMux()
	v := v{
		blobs: blobs{
			contents: map[string][]byte{},
			uploads:  map[string][]byte{},
		},
		manifests: manifests{
			manifests: map[string]map[string][]byte{},
		},
	}
	m.HandleFunc("/v2/", v.v2)
	return m
}
