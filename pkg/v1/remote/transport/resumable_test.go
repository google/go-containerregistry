package transport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	stdrand "math/rand"

	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

var rangeRe = regexp.MustCompile(`bytes=(\d+)-(\d+)?`)

func handleResumableLayer(data []byte, w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	contentRange := r.Header.Get("Range")
	if contentRange == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	matches := rangeRe.FindStringSubmatch(contentRange)
	if len(matches) != 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	contentLength := int64(len(data))
	start, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil || start < 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if start >= int64(contentLength) {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	var end = int64(contentLength) - 1
	if matches[2] != "" {
		end, err = strconv.ParseInt(matches[2], 10, 64)
		if err != nil || end < 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if end >= int64(contentLength) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
	}

	var currentContentLength = end - start + 1
	if currentContentLength <= 0 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if currentContentLength > 4096 {
		if currentContentLength = stdrand.Int63n(currentContentLength); currentContentLength < 1024 {
			currentContentLength = 1024
		}

		if r.Header.Get("X-Overlap") == "true" {
			overlapSize := int64(stdrand.Int31n(64))
			if start > overlapSize {
				start -= overlapSize
				// t.Logf("Overlap data size: %d", overlapSize)
			}
		}
	}

	end = start + currentContentLength - 1

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, contentLength))
	w.Header().Set("Content-Length", strconv.FormatInt(currentContentLength, 10))
	w.WriteHeader(http.StatusPartialContent)
	w.Write(data[start : end+1])
	time.Sleep(time.Second)
}

func resumableRequest(client *http.Client, url string, size int64, digest string, overlap bool, t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequest(): %v", err)
	}

	if overlap {
		req.Header.Set("X-Overlap", "true")
	}

	resp, err := client.Do(req.WithContext(t.Context()))
	if err != nil {
		t.Fatalf("client.Do(): %v", err)
	}
	defer resp.Body.Close()

	if _, ok := resp.Body.(*resumableBody); !ok {
		t.Error("expected resumable body")
		return
	}

	hash := sha256.New()

	if _, err = io.Copy(hash, resp.Body); err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	actualDigest := "sha256:" + hex.EncodeToString(hash.Sum(nil))

	if actualDigest != digest {
		t.Errorf("unexpected digest: %s, actually: %s", digest, actualDigest)
	}
}

func nonResumableRequest(client *http.Client, url string, t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequest(): %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do(): %v", err)
	}

	_, ok := resp.Body.(*resumableBody)
	if ok {
		t.Error("expected non-resumable body")
	}
}

func resumableStopByTimeoutRequest(client *http.Client, url string, t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequest(): %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second*3)
	defer cancel()

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatalf("client.Do(): %v", err)
	}
	defer resp.Body.Close()

	if _, ok := resp.Body.(*resumableBody); !ok {
		t.Error("expected resumable body")
		return
	}

	if _, err = io.Copy(io.Discard, resp.Body); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Error("expected context deadline exceeded error")
	}
}

func resumableStopByCancelRequest(client *http.Client, url string, t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequest(): %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(time.Second*3, cancel)

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatalf("client.Do(): %v", err)
	}
	defer resp.Body.Close()

	if _, ok := resp.Body.(*resumableBody); !ok {
		t.Error("expected resumable body")
		return
	}

	if _, err = io.Copy(io.Discard, resp.Body); err != nil && !errors.Is(err, context.Canceled) {
		t.Error("expected context cancel error")
	}
}

func TestResumableTransport(t *testing.T) {
	logs.Debug.SetOutput(os.Stdout)
	layer, err := random.Layer(2<<20, types.DockerLayer)
	if err != nil {
		t.Fatalf("random.Layer(): %v", err)
	}

	digest, err := layer.Digest()
	if err != nil {
		t.Fatalf("layer.Digest(): %v", err)
	}

	size, err := layer.Size()
	if err != nil {
		t.Fatalf("layer.Size(): %v", err)
	}

	rc, err := layer.Compressed()
	if err != nil {
		t.Fatalf("layer.Compressed(): %v", err)
	}

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("io.ReadAll(): %v", err)
	}

	layerPath := fmt.Sprintf("/v2/foo/bar/blobs/%s", digest.String())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case layerPath:
			handleResumableLayer(data, w, r, t)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	address, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	client := &http.Client{
		Transport: NewResumable(http.DefaultTransport.(*http.Transport).Clone()),
	}

	tests := []struct {
		name         string
		digest       string
		size         int64
		timeout      bool
		cancel       bool
		nonResumable bool
		overlap      bool
	}{
		{
			name:   "resumable",
			digest: digest.String(),
			size:   size,
		},
		{
			name:    "resumable-overlap",
			digest:  digest.String(),
			size:    size,
			overlap: true,
		},
		{
			name:         "non-resumable",
			nonResumable: true,
		},
		{
			name:   "resumable stop by timeout",
			cancel: true,
		},
		{
			name:   "resumable stop by cancel",
			cancel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := address.String() + layerPath
			if tt.nonResumable {
				nonResumableRequest(client, address.String(), t)
			} else if tt.cancel {
				resumableStopByCancelRequest(client, url, t)
			} else if tt.timeout {
				resumableStopByTimeoutRequest(client, url, t)
			} else if tt.digest != "" && tt.size > 0 {
				resumableRequest(client, url, tt.size, tt.digest, tt.overlap, t)
			}
		})
	}
}
