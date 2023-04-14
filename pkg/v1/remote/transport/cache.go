// Copyright 2023 Google LLC All Rights Reserved.
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

//go:build !windows
// +build !windows

package transport

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/renameio"
)

func getCache() (cache, error) {
	cachedir := os.Getenv("GGCR_CREDS_CACHE")
	if cachedir == "" {
		return nil, nil
	}
	for _, d := range []string{"ping", "token"} {
		dir := filepath.Join(cachedir, d)
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("MkdirAll(%q): %w", dir, err)
		}
	}

	return &fileCache{cachedir}, nil
}

type fileCache struct {
	dir string
}

func (c *fileCache) Get(key string) ([]byte, error) {
	fname := filepath.Join(c.dir, key)
	stat, err := os.Stat(fname)
	if err != nil {
		return nil, err
	}

	if stat.ModTime().Before(time.Now().Add(-10 * time.Minute)) {
		logs.Debug.Printf("cache found %q but it was expired", key)
		return nil, nil
	}
	return os.ReadFile(fname)
}

func (c *fileCache) Put(key string, b []byte) error {
	fname := filepath.Join(c.dir, key)

	// This library does not exist for windows.
	return renameio.WriteFile(fname, b, 0666)
}
