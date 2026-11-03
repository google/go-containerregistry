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

package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/internal/retry"
)

type mockTransport struct {
	errs  []error
	resps []*http.Response
	count int
}

func (t *mockTransport) RoundTrip(_ *http.Request) (out *http.Response, err error) {
	defer func() { t.count++ }()
	if t.count < len(t.resps) {
		out = t.resps[t.count]
	}
	if t.count < len(t.errs) {
		err = t.errs[t.count]
	}
	return
}

type perm struct{}

func (e perm) Error() string {
	return "permanent error"
}

type temp struct{}

func (e temp) Error() string {
	return "temporary error"
}

func (e temp) Temporary() bool {
	return true
}

func resp(code int) *http.Response {
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader("hi")),
	}
}

func TestRetryTransport(t *testing.T) {
	for _, test := range []struct {
		errs  []error
		resps []*http.Response
		ctx   context.Context
		count int
	}{{
		// Don't retry retry.Never.
		errs:  []error{temp{}},
		ctx:   retry.Never(context.Background()),
		count: 1,
	}, {
		// Don't retry permanent.
		errs:  []error{perm{}},
		count: 1,
	}, {
		// Do retry temp.
		errs:  []error{temp{}, perm{}},
		count: 2,
	}, {
		// Stop at some max.
		errs:  []error{temp{}, temp{}, temp{}, temp{}, temp{}},
		count: 3,
	}, {
		// Retry http errors.
		errs: []error{nil, nil, temp{}, temp{}, temp{}},
		resps: []*http.Response{
			resp(http.StatusRequestTimeout),
			resp(http.StatusInternalServerError),
			nil,
		},
		count: 3,
	}} {
		mt := mockTransport{
			errs:  test.errs,
			resps: test.resps,
		}

		tr := NewRetry(&mt,
			WithRetryBackoff(retry.Backoff{Steps: 3}),
			WithRetryPredicate(retry.IsTemporary),
			WithRetryStatusCodes(http.StatusRequestTimeout, http.StatusInternalServerError),
		)

		ctx := context.Background()
		if test.ctx != nil {
			ctx = test.ctx
		}
		req, err := http.NewRequestWithContext(ctx, "GET", "example.com", nil)
		if err != nil {
			t.Fatal(err)
		}
		tr.RoundTrip(req)
		if mt.count != test.count {
			t.Errorf("wrong count, wanted %d, got %d", test.count, mt.count)
		}
	}
}

func TestRetryDefaults(t *testing.T) {
	tr := NewRetry(http.DefaultTransport)
	rt, ok := tr.(*retryTransport)
	if !ok {
		t.Fatal("could not cast to retryTransport")
	}

	if rt.backoff != defaultBackoff {
		t.Fatalf("default backoff wrong: %v", rt.backoff)
	}

	if rt.predicate == nil {
		t.Fatal("default predicate not set")
	}
}

// TestRetryTransport_TooManyRequests covers two invariants that consumers
// (including pkg/gcrane/copy.go's outer retry loop) depend on once 429 is
// in the default retry list:
//
//  1. A 429 response is wrapped into a temporary *Error and retried up to
//     the configured Steps.
//  2. After retries exhaust, the *Error surfaced to the caller still has
//     StatusCode == 429 so callers like gcrane's hasStatusCode helper can
//     route their own application-level backoff (GCRBackoff) on top.
func TestRetryTransport_TooManyRequests(t *testing.T) {
	mt := mockTransport{
		resps: []*http.Response{
			resp(http.StatusTooManyRequests),
			resp(http.StatusTooManyRequests),
			resp(http.StatusTooManyRequests),
		},
	}

	tr := NewRetry(&mt,
		WithRetryBackoff(retry.Backoff{Steps: 3}),
		WithRetryPredicate(retry.IsTemporary),
		WithRetryStatusCodes(http.StatusTooManyRequests),
	)

	req, err := http.NewRequestWithContext(context.Background(), "GET", "example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := tr.RoundTrip(req)

	if mt.count != 3 {
		t.Errorf("expected 3 attempts (1 + 2 retries), got %d", mt.count)
	}
	if out == nil || out.StatusCode != http.StatusTooManyRequests {
		t.Errorf("final response should still surface 429 status; got %v", out)
	}
}

// TestTemporaryStatusCodes_Includes429 keeps temporaryStatusCodes (the
// raw-status fallback path) in sync with temporaryErrorCodes (the
// parsed-body path), which already contains TooManyRequestsErrorCode.
// A registry returning 429 with no structured body should still be
// classified temporary so downstream consumers retrying on
// transport.Error.Temporary() behave consistently with consumers that
// retry on a TOOMANYREQUESTS error code in the body.
func TestTemporaryStatusCodes_Includes429(t *testing.T) {
	if _, ok := temporaryStatusCodes[http.StatusTooManyRequests]; !ok {
		t.Fatal("temporaryStatusCodes should contain http.StatusTooManyRequests for parity with temporaryErrorCodes[TooManyRequestsErrorCode]")
	}
}

func TestTimeoutContext(t *testing.T) {
	tr := NewRetry(http.DefaultTransport)

	slowServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		// hanging request
		time.Sleep(time.Second * 1)
	}))
	defer func() { go func() { slowServer.Close() }() }()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond*20))
	defer cancel()
	req, err := http.NewRequest("GET", slowServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req = req.WithContext(ctx)

	result := make(chan error)

	go func() {
		_, err := tr.RoundTrip(req)
		result <- err
	}()

	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got: %v, want: %v", err, context.DeadlineExceeded)
		}
	case <-time.After(time.Millisecond * 100):
		t.Fatalf("deadline was not recognized by transport")
	}
}

// TestRetryTransportRewindsBody is a regression test for
// https://github.com/google/go-containerregistry/issues/1004: when a request
// with a body fails with a temporary network error, the retry must not reuse
// the already-consumed body. Simulate a flaky registry that kills the
// connection on the first attempt without reading the request body, then
// verify that the retried request arrives with the complete body.
func TestRetryTransportRewindsBody(t *testing.T) {
	// Large enough that the first attempt's write cannot be fully buffered,
	// guaranteeing that the failed attempt (partially) consumed the body.
	payload := bytes.Repeat([]byte("flaky-registry"), 1<<17)

	var mu sync.Mutex
	attempts := 0
	var bodies [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n == 1 {
			// Abruptly close the connection without reading the body, like a
			// registry (or LB) dropping the connection mid-upload.
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("server does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatal(err)
			}
			conn.Close()
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	tr := NewRetry(&http.Transport{},
		WithRetryBackoff(retry.Backoff{Steps: 3}),
		// The exact error for an aborted connection is platform-dependent
		// (ECONNRESET, EPIPE, EOF, ...), so retry any error here; rewinding
		// behavior is what this test cares about.
		WithRetryPredicate(retry.IsNotNil),
	)

	// http.NewRequest sets GetBody automatically for *bytes.Reader.
	req, err := http.NewRequest(http.MethodPut, server.URL+"/v2/example/blobs/uploads/123", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error (request body not rewound before retry?): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	mu.Lock()
	defer mu.Unlock()
	if attempts < 2 {
		t.Fatalf("server saw %d attempt(s), want at least 2", attempts)
	}
	if len(bodies) != 1 {
		t.Fatalf("server successfully read %d bodies, want 1", len(bodies))
	}
	if !bytes.Equal(bodies[0], payload) {
		t.Errorf("retried request body was truncated: got %d bytes, want %d", len(bodies[0]), len(payload))
	}
}

func TestRewindBody(t *testing.T) {
	t.Run("no body", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !rewindBody(req) {
			t.Error("rewindBody() = false for a request without a body, want true")
		}
	})

	t.Run("NoBody", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPut, "http://example.com", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		if !rewindBody(req) {
			t.Error("rewindBody() = false for http.NoBody, want true")
		}
	})

	t.Run("no GetBody", func(t *testing.T) {
		// io.Reader wrapping hides the concrete type, so http.NewRequest
		// cannot populate GetBody; this mimics streaming layers.
		var streaming io.Reader = strings.NewReader("streaming")
		req, err := http.NewRequest(http.MethodPut, "http://example.com", io.NopCloser(streaming))
		if err != nil {
			t.Fatal(err)
		}
		if req.GetBody != nil {
			t.Fatal("expected GetBody to be nil for this test")
		}
		if rewindBody(req) {
			t.Error("rewindBody() = true for a non-rewindable body, want false")
		}
	})

	t.Run("consumed body", func(t *testing.T) {
		payload := "some manifest"
		req, err := http.NewRequest(http.MethodPut, "http://example.com", strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		// Consume the body, as a failed first attempt would.
		if _, err := io.Copy(io.Discard, req.Body); err != nil {
			t.Fatal(err)
		}
		if !rewindBody(req) {
			t.Fatal("rewindBody() = false for a rewindable body, want true")
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != payload {
			t.Errorf("rewound body = %q, want %q", body, payload)
		}
	})
}
