package local

import (
	"errors"

	"github.com/google/go-containerregistry/pkg/v1/layout"
)

// Option is a functional option for remote operations.
type Option func(*options) error

type options struct {
	path *layout.Path
}

func makeOptions(opts ...Option) (*options, error) {
	o := &options{
		path: nil,
	}

	for _, option := range opts {
		if err := option(o); err != nil {
			return nil, err
		}
	}

	if o.path == nil {
		return nil, errors.New("provide an option for local storage")
	}

	return o, nil
}

func WithPath(p string) Option {
	return func(o *options) error {
		path, err := layout.FromPath(p)
		if err != nil {
			return err
		}
		o.path = &path
		return nil
	}
}
