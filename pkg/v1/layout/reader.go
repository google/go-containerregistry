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

package layout

// Reader provides read operations for an OCI image layout using an immutable
// per-reader policy. Readers returned without options retain Path's strict
// behavior.
type Reader struct {
	path    Path
	options readOptions
}

type readOptions struct {
	allowFinalSymlinks bool
}

// ReadOption configures a Reader when it is created.
type ReadOption interface {
	apply(*readOptions)
}

type readOptionFunc func(*readOptions)

func (f readOptionFunc) apply(options *readOptions) {
	f(options)
}

// Reader returns an immutable reader for the Path. With no options, blob reads
// reject final symlink entries exactly like the existing Path methods.
func (l Path) Reader(options ...ReadOption) Reader {
	ro := readOptions{}
	for _, option := range options {
		option.apply(&ro)
	}
	return Reader{path: l, options: ro}
}

// WithAllowFinalSymlinks allows a Reader to follow final symlink entries in the
// layout's blobs directory. Use this only for layouts produced by a
// caller-trusted local producer. Symlink targets may be outside the layout, and
// parent path components retain normal operating-system resolution; this option
// does not provide a directory sandbox or a new security guarantee. Resolved
// targets must still be regular files and must match the opened file identity.
func WithAllowFinalSymlinks() ReadOption {
	return readOptionFunc(func(options *readOptions) {
		options.allowFinalSymlinks = true
	})
}
