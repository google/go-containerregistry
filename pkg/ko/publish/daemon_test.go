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

package publish

import (
	"context"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

type MockImageLoader struct{}

func (m *MockImageLoader) ImageLoad(_ context.Context, _ io.Reader, _ bool) (types.ImageLoadResponse, error) {
	return types.ImageLoadResponse{
		Body: ioutil.NopCloser(strings.NewReader("Loaded")),
	}, nil
}

func init() {
	daemon.GetImageLoader = func() (daemon.ImageLoader, error) {
		return &MockImageLoader{}, nil
	}
}

func TestDaemon(t *testing.T) {
	importpath := "github.com/google/go-containerregistry/cmd/ko"
	img, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image() = %v", err)
	}

	def := NewDaemon(daemon.WriteOptions{})
	if d, err := def.Publish(img, importpath); err != nil {
		t.Errorf("Publish() = %v", err)
	} else if got, want := d.String(), "ko.local/"+importpath; !strings.HasPrefix(got, want) {
		t.Errorf("Publish() = %v, wanted prefix %v", got, want)
	}
}
