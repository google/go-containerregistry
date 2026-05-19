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

package remote

import (
	"net/http"
	"testing"
)

// TestDefaultRetryStatusCodes_Includes429 pins HTTP 429 (Too Many
// Requests) in the default retry status code list. The retry transport
// short-circuits on this list before consulting Temporary(), so 429 has
// to live here even though TooManyRequestsErrorCode is already classified
// temporary in transport/error.go. If a future refactor drops 429 this
// test fails loudly before the regression ships.
func TestDefaultRetryStatusCodes_Includes429(t *testing.T) {
	for _, c := range defaultRetryStatusCodes {
		if c == http.StatusTooManyRequests {
			return
		}
	}
	t.Fatal("defaultRetryStatusCodes should include http.StatusTooManyRequests so registries returning 429 are retried by the default transport")
}
