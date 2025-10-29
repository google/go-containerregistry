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
	"strings"
	"testing"

	api "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type errReader struct {
	err error
}

func (r *errReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

func (m *MockClient) ImageLoad(ctx context.Context, r io.Reader, _ ...client.ImageLoadOption) (api.LoadResponse, error) {
	if !m.negotiated {
		return api.LoadResponse{}, errors.New("you forgot to call NegotiateAPIVersion before calling ImageLoad")
	}
	if m.wantCtx != nil && m.wantCtx != ctx {
		return api.LoadResponse{}, fmt.Errorf("ImageLoad: wrong context")
	}

	_, _ = io.Copy(io.Discard, r)
	return api.LoadResponse{
		Body: m.loadBody,
	}, m.loadErr
}

func (m *MockClient) ImageTag(ctx context.Context, _, _ string) error {
	if !m.negotiated {
		return errors.New("you forgot to call NegotiateAPIVersion before calling ImageTag")
	}
	if m.wantCtx != nil && m.wantCtx != ctx {
		return fmt.Errorf("ImageTag: wrong context")
	}
	return m.tagErr
}

func TestWriteImage(t *testing.T) {
	for _, tc := range []struct {
		name         string
		client       *MockClient
		wantResponse string
		wantErr      string
	}{{
		name: "success",
		client: &MockClient{
			inspectErr: errors.New("nope"),
			loadBody:   io.NopCloser(strings.NewReader("Loaded")),
		},
		wantResponse: "Loaded",
	}, {
		name: "load err",
		client: &MockClient{
			inspectErr: errors.New("nope"),
			loadBody:   io.NopCloser(strings.NewReader("Loaded")),
			loadErr:    fmt.Errorf("locked and loaded"),
		},
		wantErr: "locked and loaded",
	}, {
		name: "read err",
		client: &MockClient{
			inspectErr: errors.New("nope"),
			loadBody:   io.NopCloser(&errReader{fmt.Errorf("goodbye, world")}),
		},
		wantErr: "goodbye, world",
	}, {
		name: "skip load",
		client: &MockClient{
			tagErr: fmt.Errorf("called tag"),
		},
		wantErr: "called tag",
	}, {
		name: "skip tag",
		client: &MockClient{
			inspectResp: inspectResp,
			tagErr:      fmt.Errorf("called tag"),
		},
		wantResponse: "",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			image, err := tarball.ImageFromPath("../tarball/testdata/test_image_1.tar", nil)
			if err != nil {
				t.Errorf("Error loading image: %v", err.Error())
			}
			tag, err := name.NewTag("test_image_2:latest")
			if err != nil {
				t.Fatal(err)
			}
			response, err := Write(tag, image, WithClient(tc.client))
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("Error writing image tar: %s", err.Error())
				}
			} else {
				if err == nil {
					t.Errorf("expected err")
				} else if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("Error writing image tar: wanted %s to contain %s", err.Error(), tc.wantErr)
				}
			}
			if !strings.Contains(response, tc.wantResponse) {
				t.Errorf("Error loading image. Response: %s", response)
			}
		})
	}
}

func TestWriteDefaultClient(t *testing.T) {
	wantErr := fmt.Errorf("bad client")
	defaultClient = func() (Client, error) {
		return nil, wantErr
	}

	tag, err := name.NewTag("test_image_2:latest")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Write(tag, empty.Image); !errors.Is(err, wantErr) {
		t.Errorf("Write(): want %v; got %v", wantErr, err)
	}

	if err := Tag(tag, tag); !errors.Is(err, wantErr) {
		t.Errorf("Tag(): want %v; got %v", wantErr, err)
	}

	// Cover default client init and ctx use as well.
	ctx := context.TODO()
	defaultClient = func() (Client, error) {
		return &MockClient{
			inspectErr: errors.New("nope"),
			loadBody:   io.NopCloser(strings.NewReader("Loaded")),
			wantCtx:    ctx,
		}, nil
	}
	if err := Tag(tag, tag, WithContext(ctx)); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(tag, empty.Image, WithContext(ctx)); err != nil {
		t.Fatal(err)
	}
}
