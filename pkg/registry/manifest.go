package registry

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type manifests struct {
	// maps container -> manifest tag/ digest -> manifest
	manifests map[string]map[string][]byte
}

func isManifest(req *http.Request) bool {
	elems := strings.Split(req.URL.Path, "/")
	elems = elems[1:]
	if len(elems) < 4 {
		return false
	}
	return elems[len(elems)-2] == "manifests"
}

// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#pulling-an-image-manifest
// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#pushing-an-image
func (m *manifests) handle(resp http.ResponseWriter, req *http.Request) {
	elem := strings.Split(req.URL.Path, "/")
	elem = elem[1:]
	// Must have a path of form /v2/{container}/manifests/{tag, digest}
	if len(elem) < 4 {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	target := elem[len(elem)-1]
	container := strings.Join(elem[1:len(elem)-2], "/")

	if req.Method == "GET" {
		if _, ok := m.manifests[container]; !ok {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		m, ok := m.manifests[container][target]
		if !ok {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		resp.Header().Set("Content-Length", fmt.Sprint(len(m)))
		resp.WriteHeader(http.StatusOK)
		io.Copy(resp, bytes.NewReader(m))
		return
	}

	if req.Method == "HEAD" {
		if _, ok := m.manifests[container]; !ok {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		m, ok := m.manifests[container][target]
		if !ok {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		resp.Header().Set("Content-Length", fmt.Sprint(len(m)))
		resp.WriteHeader(http.StatusOK)
		return
	}

	if req.Method == "PUT" {
		if _, ok := m.manifests[container]; !ok {
			m.manifests[container] = map[string][]byte{}
		}
		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
		}
		m.manifests[container][target] = b
		resp.WriteHeader(http.StatusCreated)
		return
	}
	resp.WriteHeader(http.StatusBadRequest)
}
