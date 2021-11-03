// Copyright 2019 Google LLC All Rights Reserved.
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

package remote_test

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

func TestStatusCodeReturned(t *testing.T) {
	tcs := []struct {
		Description string
		Handler     http.Handler
	}{{
		Description: "Only returns teapot status",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}),
	}, {
		Description: "Handle v2, returns teapot status else",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Print(r.URL.Path)
			if r.URL.Path == "/v2/" {
				return
			}
			w.WriteHeader(http.StatusTeapot)
		}),
	}}

	for _, tc := range tcs {
		t.Run(tc.Description, func(t *testing.T) {
			o := httptest.NewServer(tc.Handler)
			defer o.Close()

			ref, err := name.NewDigest(strings.TrimPrefix(o.URL+"/foo:@sha256:53b27244ffa2f585799adbfaf79fba5a5af104597751b289c8b235e7b8f7ebf5", "http://"))

			if err != nil {
				t.Fatalf("Unable to parse digest: %v", err)
			}

			_, err = remote.Image(ref)
			var terr *transport.Error
			if !errors.As(err, &terr) {
				t.Fatalf("Unable to cast error to transport error: %v", err)
			}
			if terr.StatusCode != http.StatusTeapot {
				t.Errorf("Incorrect status code received, got %v, wanted %v", terr.StatusCode, http.StatusTeapot)
			}
		})
	}
}

func TestBlobStatusCodeReturned(t *testing.T) {
	reg := registry.New()
	rh := httptest.NewServer(reg)
	defer rh.Close()
	i, _ := random.Image(1024, 16)
	tag := strings.TrimPrefix(fmt.Sprintf("%s/foo:bar", rh.URL), "http://")
	d, _ := name.NewTag(tag)
	if err := remote.Write(d, i); err != nil {
		t.Fatalf("Unable to write empty image: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Print(r.URL.Path)
		if strings.Contains(r.URL.Path, "blob") {
			w.WriteHeader(http.StatusTeapot)
			return
		}
		reg.ServeHTTP(w, r)
	})

	o := httptest.NewServer(handler)
	defer o.Close()

	ref, err := name.NewTag(strings.TrimPrefix(fmt.Sprintf("%s/foo:bar", o.URL), "http://"))
	if err != nil {
		t.Fatalf("Unable to parse digest: %v", err)
	}

	ri, err := remote.Image(ref)
	if err != nil {
		t.Fatalf("Unable to fetch manifest: %v", err)
	}
	l, err := ri.Layers()
	if err != nil {
		t.Fatalf("Unable to fetch layers: %v", err)
	}
	_, err = l[0].Compressed()
	var terr *transport.Error
	if !errors.As(err, &terr) {
		t.Fatalf("Unable to cast error to transport error: %v", err)
	}
	if terr.StatusCode != http.StatusTeapot {
		t.Errorf("Incorrect status code received, got %v, wanted %v", terr.StatusCode, http.StatusTeapot)
	}
	_, err = l[0].Uncompressed()
	if !errors.As(err, &terr) {
		t.Fatalf("Unable to cast error to transport error: %v", err)
	}
	if terr.StatusCode != http.StatusTeapot {
		t.Errorf("Incorrect status code received, got %v, wanted %v", terr.StatusCode, http.StatusTeapot)
	}
}
