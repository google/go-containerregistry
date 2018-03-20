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
	"io"
	"io/ioutil"
	"strings"
	"testing"
)

func compressData(t *testing.T, data string) io.Reader {
	buf := bytes.Buffer{}
	gz := gzip.NewWriter(&buf)
	defer gz.Close()
	if _, err := gz.Write([]byte(data)); err != nil {
		t.Fatalf("Error setting up compressed data.")
	}
	return &buf
}

func decompressData(t *testing.T, r io.Reader) string {
	gz, err := gzip.NewReader(r)
	if err != nil {
		t.Fatalf("Error reading compressed data: %v", err)
	}
	data, err := ioutil.ReadAll(gz)
	if err != nil {
		t.Fatalf("Error reading compressed data: %v", err)
	}
	return string(data)
}

func TestEnsureCompressed(t *testing.T) {

	type args struct {
		r io.Reader
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "already compressed",
			args: args{r: compressData(t, "some data")},
			want: "some data",
		},
		{
			name: "not compressed",
			args: args{r: strings.NewReader("some data")},
			want: "some data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnsureCompressed(tt.args.r)
			if err != nil {
				t.Errorf("Unexpected error = %v from EnsureCompressed()", err)
			}
			decompressed := decompressData(t, got)
			if decompressed != tt.want {
				t.Errorf("EnsureCompressed() got = %v, want %v", decompressed, tt.want)
			}
		})
	}
}
