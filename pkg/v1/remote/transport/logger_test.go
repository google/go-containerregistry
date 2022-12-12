// Copyright 2019 Google LLC All Rights Reserved.
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
	"strings"
	"testing"

	"github.com/google/go-containerregistry/internal/redact"
	"github.com/google/go-containerregistry/pkg/logs"
)

func TestLogger(t *testing.T) {
	canary := "logs.Debug canary"
	secret := "super secret do not log"
	auth := "my token pls do not log"
	reason := "should not log the secret"

	ctx := redact.NewContext(context.Background(), reason)

	req, err := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Unexpected error during NewRequest: %v", err)
	}
	req.Header.Set("authorization", auth)

	var b bytes.Buffer
	logs.Debug.SetOutput(&b)
	cannedResponse := http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Foo": []string{canary},
		},
		Body:    io.NopCloser(strings.NewReader(secret)),
		Request: req,
	}
	tr := NewLogger(newRecorder(&cannedResponse, nil))
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("Unexpected error during RoundTrip: %v", err)
	}

	logged := b.String()
	if !strings.Contains(logged, canary) {
		t.Errorf("Expected logs to contain %s, got %s", canary, logged)
	}
	if !strings.Contains(logged, reason) {
		t.Errorf("Expected logs to contain %s, got %s", canary, logged)
	}
	if strings.Contains(logged, secret) {
		t.Errorf("Expected logs NOT to contain %s, got %s", secret, logged)
	}
	if strings.Contains(logged, auth) {
		t.Errorf("Expected logs NOT to contain %s, got %s", auth, logged)
	}
}

func TestLoggerError(t *testing.T) {
	canary := "logs.Debug canary ERROR"
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Unexpected error during NewRequest: %v", err)
	}

	var b bytes.Buffer
	logs.Debug.SetOutput(&b)
	tr := NewLogger(newRecorder(nil, errors.New(canary)))
	if _, err := tr.RoundTrip(req); err == nil {
		t.Fatalf("Expected error during RoundTrip, got nil")
	}

	logged := b.String()
	if !strings.Contains(logged, canary) {
		t.Errorf("Expected logs to contain %s, got %s", canary, logged)
	}
}
