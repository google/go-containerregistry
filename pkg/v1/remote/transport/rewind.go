// Copyright 2026 Google LLC All Rights Reserved.
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
	"net/http"

	"github.com/google/go-containerregistry/pkg/logs"
)

// rewindBody attempts to restore in.Body so that the request can be sent
// again after a previous attempt may have (partially or fully) consumed it.
// It mirrors what net/http does when it transparently retries a request
// (see rewindBody in net/http/transport.go).
//
// Without this, resending a request whose body was already consumed makes
// the retry fail with errors like:
//
//	http: ContentLength=1821 with Body length 0
//
// See https://github.com/google/go-containerregistry/issues/1004.
//
// A fresh body is obtained from in.GetBody, which http.NewRequest populates
// automatically for common body types (*bytes.Buffer, *bytes.Reader,
// *strings.Reader) — covering manifest PUTs — and which we set explicitly
// for blob uploads of non-streaming layers.
//
// It reports whether in.Body is safe to send again. If the request has no
// body there is nothing to rewind and it returns true. If the body cannot
// be regenerated (GetBody is nil, e.g. for streaming layers), it returns
// false and leaves the request unchanged; callers may still choose to
// resend the request, matching previous behavior.
func rewindBody(in *http.Request) bool {
	if in.Body == nil || in.Body == http.NoBody {
		// No body to rewind.
		return true
	}
	if in.GetBody == nil {
		return false
	}
	body, err := in.GetBody()
	if err != nil {
		logs.Debug.Printf("rewinding %s %s body: %v", in.Method, in.URL, err)
		return false
	}
	// Best-effort close of the (partially) consumed body.
	in.Body.Close()
	in.Body = body
	return true
}
