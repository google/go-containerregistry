// Copyright 2022 Google LLC All Rights Reserved.
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

// Package compression abstracts over gzip and zstd.
package compression

import "errors"

// Compression is an enumeration of the supported compression algorithms
type Compression string

// The collection of known MediaType values.
const (
	None Compression = "none"
	GZip Compression = "gzip"
	ZStd Compression = "zstd"
)

// Used by fmt.Print and Cobra in help text
func (e *Compression) String() string {
	return string(*e)
}

func (e *Compression) Set(v string) error {
	switch v {
	case "none", "gzip", "zstd":
		*e = Compression(v)
		return nil
	default:
		return errors.New(`must be one of "none", "gzip, or "zstd"`)
	}
}

// Used in Cobra help text
func (e *Compression) Type() string {
	return "Compression"
}

var ErrZStdNonOci = errors.New("ZSTD compression can only be used with an OCI base image")
