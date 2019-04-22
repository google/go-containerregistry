package registry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
)

// Returns whether this url should be handled by the blob handler
// This is complicated because blob is indicated by the trailing path, not the leading path.
// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#pulling-a-layer
// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#pushing-a-layer
func isBlob(req *http.Request) bool {
	elem := strings.Split(req.URL.Path, "/")
	elem = elem[1:]
	if len(elem) < 3 {
		return false
	}
	return elem[len(elem)-2] == "blobs" || (elem[len(elem)-3] == "blobs" &&
		elem[len(elem)-2] == "uploads")
}

// blobs
type blobs struct {
	// Blobs are content addresses. we store them globally underneath their sha and make no distinctions per image.
	contents map[string][]byte
	// Each upload gets a unique id that writes occur to until finalized.
	uploads map[string][]byte
}

func (b *blobs) handle(resp http.ResponseWriter, req *http.Request) {
	elem := strings.Split(req.URL.Path, "/")
	elem = elem[1:]
	// Must have a path of form /v2/{name}/blobs/{upload,sha256:}
	if len(elem) < 4 {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	target := elem[len(elem)-1]
	service := elem[len(elem)-2]

	if req.Method == "HEAD" {
		b, ok := b.contents[target]
		if !ok {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		resp.Header().Set("Content-Length", fmt.Sprint(len(b)))
		resp.Header().Set("Docker-Content-Digest", target)
		resp.WriteHeader(http.StatusOK)
		return
	}

	if req.Method == "GET" {
		b, ok := b.contents[target]
		if !ok {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		resp.Header().Set("Content-Length", fmt.Sprint(len(b)))
		resp.Header().Set("Docker-Content-Digest", target)
		resp.WriteHeader(http.StatusOK)
		io.Copy(resp, bytes.NewReader(b))
		return
	}

	if req.Method == "POST" && target == "uploads" {
		id := fmt.Sprint(rand.Int63())
		resp.Header().Set("Location", fmt.Sprintf("/v2/%s/blobs/uploads/%s",
			strings.Join(elem[1:len(elem)-2], ","),
			id))
		resp.Header().Set("Range", "0-0")
		resp.WriteHeader(http.StatusAccepted)
		return
	}

	if req.Method == "PUT" && service == "uploads" {
		digest := req.URL.Query().Get("digest")
		if digest == "" {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}

		l := &bytes.Buffer{}
		io.Copy(l, req.Body)
		rd := sha256.Sum256(l.Bytes())
		d := "sha256:" + hex.EncodeToString(rd[:])
		if d != digest {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}

		b.contents[d] = l.Bytes()
		resp.Header().Set("Docker-Content-Digest", d)
		resp.WriteHeader(http.StatusCreated)
		return
	}

	resp.WriteHeader(http.StatusBadRequest)
	return
}
