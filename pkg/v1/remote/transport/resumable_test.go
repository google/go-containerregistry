package transport

import (
	"bytes"
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
	"strconv"
	"testing"
	"time"

	stdrand "math/rand"

	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func handleResumableLayer(data []byte, w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var (
		contentLength, start, end int64
		statusCode                = http.StatusOK
		err                       error
	)

	contentLength = int64(len(data))
	end = contentLength - 1
	contentRange := r.Header.Get("Range")
	if contentRange != "" {
		matches := rangeRe.FindStringSubmatch(contentRange)
		if len(matches) != 3 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if start, err = strconv.ParseInt(matches[1], 10, 64); err != nil || start < 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if start >= int64(contentLength) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}

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

		statusCode = http.StatusPartialContent
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

	if statusCode == http.StatusPartialContent {
		w.Header().Set("Content-Length", strconv.FormatInt(currentContentLength, 10))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, contentLength))
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}

	w.WriteHeader(statusCode)
	w.Write(data[start : end+1])
	time.Sleep(time.Second)
}

func resumableRequest(client *http.Client, url string, leading, trailing []byte, size int64, digest string, overlap bool, t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("http.NewRequest(): %v", err)
	}

	if overlap {
		req.Header.Set("X-Overlap", "true")
	}

	if len(leading) > 0 || len(trailing) > 0 {
		var buf bytes.Buffer
		buf.WriteString("bytes=")
		buf.WriteString(fmt.Sprintf("%d-", len(leading)))
		if len(trailing) > 0 {
			buf.WriteString(fmt.Sprintf("%d", size-int64(len(trailing))-1))
		}
		req.Header.Set("Range", buf.String())
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
	if len(leading) > 0 {
		io.Copy(hash, bytes.NewReader(leading))
	}

	if _, err = io.Copy(hash, resp.Body); err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if len(trailing) > 0 {
		io.Copy(hash, bytes.NewReader(trailing))
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
		Transport: NewResumable(http.DefaultTransport.(*http.Transport).Clone(), Backoff{
			Duration: 1.0 * time.Second,
			Factor:   3.0,
			Jitter:   0.1,
			Steps:    3,
		}),
	}

	tests := []struct {
		name              string
		digest            string
		leading, trailing int64
		timeout           bool
		cancel            bool
		nonResumable      bool
		overlap           bool
		ranged            bool
	}{
		{
			name:    "resumable",
			digest:  digest.String(),
			leading: 0,
		},
		{
			name:    "resumable-range-leading",
			digest:  digest.String(),
			leading: 3,
		},
		{
			name:    "resumable-range-trailing",
			digest:  digest.String(),
			leading: 0,
		},
		{
			name:     "resumable-range-leading-trailing",
			digest:   digest.String(),
			leading:  3,
			trailing: 6,
		},
		{
			name:    "resumable-overlap",
			digest:  digest.String(),
			leading: 0,
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
			} else if tt.digest != "" {
				resumableRequest(client, url, data[:tt.leading], data[size-tt.trailing:], size, tt.digest, tt.overlap, t)
			}
		})
	}
}
