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

package gcrane

import (
	"context"
	"net/http"
	"runtime"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Option is a functional option for gcrane operations.
type Option func(*options)

type options struct {
	jobs   int
	remote []remote.Option
	google []google.Option
	crane  []crane.Option
}

func makeOptions(opts ...Option) *options {
	o := &options{
		jobs: runtime.GOMAXPROCS(0),
		remote: []remote.Option{
			remote.WithAuthFromKeychain(Keychain),
		},
		google: []google.Option{
			google.WithAuthFromKeychain(Keychain),
		},
		crane: []crane.Option{
			crane.WithAuthFromKeychain(Keychain),
		},
	}

	for _, option := range opts {
		option(o)
	}

	return o
}

// WithJobs sets the number of concurrent jobs to run.
//
// The default number of jobs is GOMAXPROCS.
func WithJobs(jobs int) Option {
	return func(o *options) {
		o.jobs = jobs
	}
}

// WithTransport is a functional option for overriding the default transport
// for remote operations.
func WithTransport(t http.RoundTripper) Option {
	return func(o *options) {
		o.remote = append(o.remote, remote.WithTransport(t))
		o.google = append(o.google, google.WithTransport(t))
		o.crane = append(o.crane, crane.WithTransport(t))
	}
}

// WithUserAgent adds the given string to the User-Agent header for any HTTP
// requests.
func WithUserAgent(ua string) Option {
	return func(o *options) {
		o.remote = append(o.remote, remote.WithUserAgent(ua))
		o.google = append(o.google, google.WithUserAgent(ua))
		o.crane = append(o.crane, crane.WithUserAgent(ua))
	}
}

// WithContext is a functional option for setting the context.
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.remote = append(o.remote, remote.WithContext(ctx))
		o.google = append(o.google, google.WithContext(ctx))
		o.crane = append(o.crane, crane.WithContext(ctx))
	}
}

// WithKeychain is a functional option for overriding the default
// authenticator for remote operations, using an authn.Keychain to find
// credentials.
//
// By default, gcrane will use gcrane.Keychain.
func WithKeychain(keys authn.Keychain) Option {
	return func(o *options) {
		// Replace the default keychain at position 0.
		o.remote[0] = remote.WithAuthFromKeychain(keys)
		o.google[0] = google.WithAuthFromKeychain(keys)
		o.crane[0] = crane.WithAuthFromKeychain(keys)
	}
}

// WithAuth is a functional option for overriding the default authenticator
// for remote operations.
//
// By default, gcrane will use gcrane.Keychain.
func WithAuth(auth authn.Authenticator) Option {
	return func(o *options) {
		// Replace the default keychain at position 0.
		o.remote[0] = remote.WithAuth(auth)
		o.google[0] = google.WithAuth(auth)
		o.crane[0] = crane.WithAuth(auth)
	}
}
