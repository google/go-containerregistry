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
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTemporary(t *testing.T) {
	tests := []struct {
		error *Error
		retry bool
	}{{
		error: &Error{},
		retry: false,
	}, {
		error: &Error{
			Errors: []Diagnostic{{
				Code: BlobUploadInvalidErrorCode,
			}},
		},
		retry: true,
	}, {
		error: &Error{
			Errors: []Diagnostic{{
				Code: BlobUploadInvalidErrorCode,
			}, {
				Code: DeniedErrorCode,
			}},
		},
		retry: false,
	}, {
		error: &Error{
			Errors: []Diagnostic{{
				Code: TooManyRequestsErrorCode,
			}},
		},
		retry: true,
	}, {
		error: &Error{
			Errors: []Diagnostic{{
				Code: UnavailableErrorCode,
			}},
		},
		retry: true,
	}, {
		error: &Error{
			StatusCode: http.StatusInternalServerError,
		},
		retry: true,
	}}

	for _, test := range tests {
		retry := test.error.Temporary()

		if test.retry != retry {
			t.Errorf("Temporary(%s) = %t, wanted %t", test.error, retry, test.retry)
		}
	}
}

func TestCheckErrorNil(t *testing.T) {
	tests := []int{
		http.StatusOK,
		http.StatusAccepted,
		http.StatusCreated,
		http.StatusMovedPermanently,
		http.StatusInternalServerError,
	}

	for _, code := range tests {
		resp := &http.Response{StatusCode: code}

		if err := CheckError(resp, code); err != nil {
			t.Errorf("CheckError(%d) = %v", code, err)
		}
	}
}

func TestCheckErrorNotError(t *testing.T) {
	tests := []struct {
		code    int
		body    string
		msg     string
		request *http.Request
	}{{
		code: http.StatusBadRequest,
		body: "",
		msg:  "unexpected status code 400 Bad Request",
	}, {
		code: http.StatusUnauthorized,
		// Valid JSON, but not a structured error -- we should still print the body.
		body: `{"details":"incorrect username or password"}`,
		msg:  `unexpected status code 401 Unauthorized: {"details":"incorrect username or password"}`,
	}, {
		code: http.StatusUnauthorized,
		body: "Not JSON",
		msg:  "GET https://example.com/somepath?access_token=REDACTED&scope=foo&service=bar: unexpected status code 401 Unauthorized: Not JSON",
		request: &http.Request{
			Method: http.MethodGet,
			URL: &url.URL{
				Scheme: "https",
				Host:   "example.com",
				Path:   "somepath",
				RawQuery: url.Values{
					"scope":        []string{"foo"},
					"service":      []string{"bar"},
					"access_token": []string{"hunter2"},
				}.Encode(),
			},
		},
	}, {
		code: http.StatusUnauthorized,
		body: "",
		msg:  "HEAD https://example.com/somepath: unexpected status code 401 Unauthorized (HEAD responses have no body, use GET for details)",
		request: &http.Request{
			Method: http.MethodHead,
			URL: &url.URL{
				Scheme: "https",
				Host:   "example.com",
				Path:   "somepath",
			},
		},
	}}

	for _, test := range tests {
		resp := &http.Response{
			StatusCode: test.code,
			Body:       io.NopCloser(bytes.NewBufferString(test.body)),
			Request:    test.request,
		}

		err := CheckError(resp, http.StatusOK)
		if err == nil {
			t.Fatalf("CheckError(%d, %s) = nil, wanted error", test.code, test.body)
		}
		var terr *Error
		if !errors.As(err, &terr) {
			t.Fatalf("CheckError(%d, %s) = %v, wanted error type", test.code, test.body, err)
		}

		if terr.StatusCode != test.code {
			t.Errorf("Incorrect status code, got %d, want %d", terr.StatusCode, test.code)
		}

		if terr.Error() != test.msg {
			t.Errorf("Incorrect message, got %q, want %q", terr.Error(), test.msg)
		}
	}
}

func TestCheckErrorWithError(t *testing.T) {
	tests := []struct {
		name      string
		code      int
		errorBody string
		msg       string
	}{{
		name:      "Invalid name error",
		code:      http.StatusBadRequest,
		errorBody: `{"errors":[{"code":"NAME_INVALID","message":"a message for you"}],"StatusCode":400}`,
		msg:       "NAME_INVALID: a message for you",
	}, {
		name:      "Only status code is provided",
		code:      http.StatusBadRequest,
		errorBody: `{"StatusCode":400}`,
		msg:       "unexpected status code 400 Bad Request: {\"StatusCode\":400}",
	}, {
		name:      "Multiple diagnostics",
		code:      http.StatusBadRequest,
		errorBody: `{"errors":[{"code":"NAME_INVALID","message":"a message for you"}, {"code":"SIZE_INVALID","message":"another message for you", "detail": "with some details"}],"StatusCode":400,"Request":null}`,
		msg:       "multiple errors returned: NAME_INVALID: a message for you; SIZE_INVALID: another message for you; with some details",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: test.code,
				Body:       io.NopCloser(bytes.NewBuffer([]byte(test.errorBody))),
			}

			var terr *Error
			if err := CheckError(resp, http.StatusOK); err == nil {
				t.Errorf("CheckError(%d, %s) = nil, wanted error", test.code, test.errorBody)
			} else if !errors.As(err, &terr) {
				t.Errorf("CheckError(%d, %s) = %T, wanted *transport.Error", test.code, test.errorBody, err)
			} else if diff := cmp.Diff(test.msg, err.Error()); diff != "" {
				t.Errorf("CheckError(%d, %s).Error(); (-want +got) %s", test.code, test.errorBody, diff)
			}
		})
	}
}

func TestBodyError(t *testing.T) {
	expectedErr := errors.New("whoops")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       &errReadCloser{expectedErr},
	}
	if err := CheckError(resp, http.StatusNotFound); err == nil {
		t.Errorf("CheckError() = nil, wanted error %v", expectedErr)
	} else if !errors.Is(err, expectedErr) {
		t.Errorf("CheckError() = %v, wanted %v", err, expectedErr)
	}
}

type errReadCloser struct {
	err error
}

func (e *errReadCloser) Read(p []byte) (int, error) {
	return 0, e.err
}

func (e *errReadCloser) Close() error {
	return e.err
}
