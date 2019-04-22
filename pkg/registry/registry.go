// Package registry implements a docker V2 registry.
package registry

import (
	"net/http"
)

// https://docs.docker.com/registry/spec/api/#api-version-check
// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#api-version-check
func version(resp http.ResponseWriter, req *http.Request) {
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
	m.HandleFunc("/v2/", version)
	return m
}
