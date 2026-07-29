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

package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
)

func TestExportDoesNotCreateDestinationWhenPullFails(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.NotFoundHandler())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "missing.tar")
	options := []crane.Option{crane.Insecure}
	cmd := NewCmdExport(&options)

	err = cmd.RunE(cmd, []string{fmt.Sprintf("%s/missing:latest", u.Host), dst})
	if err == nil {
		t.Fatal("NewCmdExport returned nil error, want pull failure")
	}
	if !strings.Contains(err.Error(), "pulling") {
		t.Fatalf("NewCmdExport error = %v, want pulling error", err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("destination file exists after pull failure: %v", err)
	}
}
