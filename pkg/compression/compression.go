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

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Compression is an enumeration of the supported compression algorithms
type Compression string

// The collection of known MediaType values.
const (
	None Compression = "none"
	GZip Compression = "gzip"
	ZStd Compression = "zstd"
)

func (compression Compression) ToMediaType(oci bool) (types.MediaType, error) {
	if oci {
		switch compression {
		case ZStd:
			return types.OCILayerZStd, nil
		case GZip:
			return types.OCILayer, nil
		case None:
			return types.OCIUncompressedLayer, nil
		default:
			return types.OCILayer, fmt.Errorf("unsupported compression: %s", compression)
		}
	} else {
		switch compression {
		case ZStd:
			return types.DockerLayerZstd, nil
		case GZip:
			return types.DockerLayer, nil
		case None:
			return types.DockerUncompressedLayer, nil
		default:
			return types.DockerLayer, fmt.Errorf("unsupported compression: %s", compression)
		}
	}
}
