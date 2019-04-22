// Package registry implements a docker V2 registry.
package registry

import (
	"net/http"
)

type v struct {
	blobs blobs
}

// https://docs.docker.com/registry/spec/api/#api-version-check
// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#api-version-check
func (v *v) v2(resp http.ResponseWriter, req *http.Request) {
	if isBlob(req) {
		v.blobs.handle(resp, req)
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
	}
	m.HandleFunc("/v2/", v.v2)
	return m
}
