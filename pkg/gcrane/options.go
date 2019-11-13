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
