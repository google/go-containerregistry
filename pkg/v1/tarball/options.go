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

package tarball

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Option is a functional option for tarball operations.
type Option func(*options) error

// LayerFilter defines a function for filtering layers.
//  True - indicates the layer should be kept,
//  False - indicates the layer should be excluded.
type LayerFilter func(v1.Layer) (bool, error)

type options struct {
	filter LayerFilter
}

func makeOptions(opts ...Option) (*options, error) {
	o := &options{
		filter: func(v1.Layer) (bool, error) {
			return true, nil
		},
	}

	for _, option := range opts {
		if err := option(o); err != nil {
			return nil, err
		}
	}

	return o, nil
}

// WithLayerFilter allows omitting layers when writing a tarball.
func WithLayerFilter(lf LayerFilter) Option {
	return func(o *options) error {
		o.filter = lf
		return nil
	}
}
