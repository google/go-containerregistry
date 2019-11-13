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
	"runtime"
)

// Option is a functional option for gcrane operations.
type Option func(*options)

type options struct {
	jobs int
}

func makeOptions(opts ...Option) *options {
	o := &options{
		jobs: runtime.GOMAXPROCS(0),
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
