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

// Package registry implements a docker V2 registry and the OCI distribution specification.
//
// It is designed to be used anywhere a low dependency container registry is needed, with an
// initial focus on tests.
//
// Its goal is to be standards compliant and its strictness will increase over time.
//
// This is currently a low flightmiles system. It's likely quite safe to use in tests; If you're using it
// in production, please let us know how and send us CL's for integration tests.
package registry

import (
	"log"
	"net/http"
)

type registry struct {
	log       *log.Logger
	blobs     blobs
	manifests manifests
}

// https://docs.docker.com/registry/spec/api/#api-version-check
// https://github.com/opencontainers/distribution-spec/blob/master/spec.md#api-version-check
func (r *registry) v2(resp http.ResponseWriter, req *http.Request) *regError {
	if isBlob(req) {
		return r.blobs.handle(resp, req)
	}
	if isManifest(req) {
		return r.manifests.handle(resp, req)
	}
	resp.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	if req.URL.Path != "/v2/" && req.URL.Path != "/v2" {
		return &regError{
			Status:  http.StatusNotFound,
			Code:    "METHOD_UNKNOWN",
			Message: "We don't understand your method + url",
		}
	}
	resp.WriteHeader(200)
	return nil
}

func (r *registry) root(resp http.ResponseWriter, req *http.Request) {
	if rerr := r.v2(resp, req); rerr != nil {
		r.logf("%s %s %d %s %s", req.Method, req.URL, rerr.Status, rerr.Code, rerr.Message)
		rerr.Write(resp)
		return
	}
	r.logf("%s %s", req.Method, req.URL)
}

func (r *registry) logf(f string, v ...interface{}) {
	if r.log == nil {
		log.Printf(f, v...)
	} else {
		r.log.Printf(f, v...)
	}
}

// New returns a handler which implements the docker registry protocol.
// It should be registered at the site root.
func New() http.Handler {
	return NewWithOptions(nil)
}

// Options describes the available options
// for creating the registry.
type Options struct {
	// Log is used to log requests.
	// If nil, the global logger is used.
	Log *log.Logger
}

// NewWithOptions is the same as New but takes options.
func NewWithOptions(opts *Options) http.Handler {
	if opts == nil {
		opts = &Options{}
	}
	v := registry{
		log: opts.Log,
		blobs: blobs{
			contents: map[string][]byte{},
			uploads:  map[string][]byte{},
		},
		manifests: manifests{
			manifests: map[string]map[string]manifest{},
		},
	}
	return http.HandlerFunc(v.root)
}
