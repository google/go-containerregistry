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

package v1util

import (
	"io"
)

// ReadAndCloser implements io.ReadCloser by reading from a particular io.Reader
// and then calling the provided "Close()" method.
type ReadAndCloser struct {
	Reader io.Reader
	Closer func() error
}

var _ io.ReadCloser = (*ReadAndCloser)(nil)

// Read implements io.ReadCloser
func (rac *ReadAndCloser) Read(p []byte) (n int, err error) {
	return rac.Reader.Read(p)
}

// Close implements io.ReadCloser
func (rac *ReadAndCloser) Close() error {
	return rac.Closer()
}

// WriteAndCloser implements io.WriteCloser by reading from a particular io.Writer
// and then calling the provided "Close()" method.
type WriteAndCloser struct {
	Writer io.Writer
	Closer func() error
}

var _ io.WriteCloser = (*WriteAndCloser)(nil)

// Write implements io.WriteCloser
func (wac *WriteAndCloser) Write(p []byte) (n int, err error) {
	return wac.Writer.Write(p)
}

// Close implements io.WriteCloser
func (wac *WriteAndCloser) Close() error {
	return wac.Closer()
}
