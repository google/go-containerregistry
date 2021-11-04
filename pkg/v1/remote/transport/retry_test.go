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
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-containerregistry/internal/retry"
)

type mockTransport struct {
	errs  []error
	count int
}

func (t *mockTransport) RoundTrip(in *http.Request) (out *http.Response, err error) {
	defer func() { t.count++ }()
	return nil, t.errs[t.count]
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

func TestRetryTransport(t *testing.T) {
	for _, test := range []struct {
		errs  []error
		count int
	}{{
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
	}} {
		mt := mockTransport{
			errs: test.errs,
		}

		tr := NewRetry(&mt, WithRetryBackoff(retry.Backoff{Steps: 3}), WithRetryPredicate(retry.IsTemporary))

		tr.RoundTrip(nil)
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

func TestTimeoutContext(t *testing.T) {
	tr := NewRetry(http.DefaultTransport)

	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
