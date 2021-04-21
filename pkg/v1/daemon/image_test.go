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

package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/internal/compare"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

var imagePath = "../tarball/testdata/test_image_1.tar"

type MockClient struct {
	Client
	path       string
	negotiated bool

	wantCtx context.Context

	loadErr  error
	loadBody io.ReadCloser

	saveErr  error
	saveBody io.ReadCloser
}

func (m *MockClient) NegotiateAPIVersion(ctx context.Context) {
	m.negotiated = true
}

func (m *MockClient) ImageSave(_ context.Context, _ []string) (io.ReadCloser, error) {
	if !m.negotiated {
		return nil, errors.New("you forgot to call NegotiateAPIVersion before calling ImageSave")
	}

	if m.path != "" {
		return os.Open(m.path)
	}

	return m.saveBody, m.saveErr
}

func TestImage(t *testing.T) {
	for _, tc := range []struct {
		name         string
		buffered     bool
		client       *MockClient
		wantResponse string
		wantErr      string
	}{{
		name: "success",
		client: &MockClient{
			path: imagePath,
		},
	}, {
		name: "save err",
		client: &MockClient{
			saveBody: ioutil.NopCloser(strings.NewReader("Loaded")),
			saveErr:  fmt.Errorf("locked and loaded"),
		},
		wantErr: "locked and loaded",
	}, {
		name: "read err",
		client: &MockClient{
			saveBody: ioutil.NopCloser(&errReader{fmt.Errorf("goodbye, world")}),
		},
		wantErr: "goodbye, world",
	}} {
		run := func(t *testing.T) {
			opts := []Option{WithClient(tc.client)}
			if tc.buffered {
				opts = append(opts, WithBufferedOpener())
			} else {
				opts = append(opts, WithUnbufferedOpener())
			}
			img, err := tarball.ImageFromPath(imagePath, nil)
			if err != nil {
				t.Fatalf("error loading test image: %s", err)
			}

			tag, err := name.NewTag("unused", name.WeakValidation)
			if err != nil {
				t.Fatalf("error creating test name: %s", err)
			}

			dmn, err := Image(tag, opts...)
			if err != nil {
				if tc.wantErr == "" {
					t.Errorf("Error loading daemon image: %s", err)
				} else if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("wanted %s to contain %s", err.Error(), tc.wantErr)
				}
				return
			}
			if err := compare.Images(img, dmn); err != nil {
				t.Errorf("compare.Images: %v", err)
			}
		}

		tc.buffered = true
		t.Run(tc.name+" buffered", run)

		tc.buffered = false
		t.Run(tc.name+" unbuffered", run)
	}
}

func TestImageDefaultClient(t *testing.T) {
	wantErr := fmt.Errorf("bad client")
	defaultClient = func() (Client, error) {
		return nil, wantErr
	}

	_, err := Image(name.MustParseReference("unused"))
	if err != wantErr {
		t.Errorf("Image(): want %v; got %v", wantErr, err)
	}
}
