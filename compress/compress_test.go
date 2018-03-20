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

package compress

import (
	"bytes"
	"compress/gzip"
	"testing"
)

func TestIsCompressed(t *testing.T) {
	data := []byte("some data")
	compressed, err := IsCompressed(bytes.NewReader(data))
	if err != nil {
		t.Errorf("Error detecting compression: %v", err)
	}
	if compressed {
		t.Errorf("Expected IsCompressed() = false, got %v", compressed)
	}

	buf := bytes.Buffer{}
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		t.Fatalf("Error zipping data: %v", err)
	}
	gz.Close()

	compressed, err = IsCompressed(&buf)
	if err != nil {
		t.Errorf("Error detecting compression: %v", err)
	}
	if !compressed {
		t.Errorf("Expected IsCompressed() = true, got %v", compressed)
	}
}
