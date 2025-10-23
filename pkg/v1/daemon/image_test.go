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
	"os"
	"strings"
	"testing"

	api "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/storage"
	"github.com/docker/docker/client"
	specs "github.com/moby/docker-image-spec/specs-go/v1"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/compare"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

var imagePath = "../tarball/testdata/test_image_1.tar"

var inspectResp = api.InspectResponse{
	ID: "sha256:6e0b05049ed9c17d02e1a55e80d6599dbfcce7f4f4b022e3c673e685789c470e",
	RepoTags: []string{
		"bazel/v1/tarball:test_image_1",
		"test_image_2:latest",
	},
	Created:      "1970-01-01T00:00:00Z",
	Author:       "Bazel",
	Architecture: "amd64",
	Os:           "linux",
	Size:         8,
	VirtualSize:  8,
	Config:       &specs.DockerOCIImageConfig{},
	GraphDriver: storage.DriverData{
		Data: map[string]string{
			"MergedDir": "/var/lib/docker/overlay2/988ecd005d048fd47b241dd57687231859563ba65a1dfd01ae1771ebfc4cb7c5/merged",
			"UpperDir":  "/var/lib/docker/overlay2/988ecd005d048fd47b241dd57687231859563ba65a1dfd01ae1771ebfc4cb7c5/diff",
			"WorkDir":   "/var/lib/docker/overlay2/988ecd005d048fd47b241dd57687231859563ba65a1dfd01ae1771ebfc4cb7c5/work",
		},
		Name: "overlay2",
	},
	RootFS: api.RootFS{
		Type: "layers",
		Layers: []string{
			"sha256:8897395fd26dc44ad0e2a834335b33198cb41ac4d98dfddf58eced3853fa7b17",
		},
	},
}

type MockClient struct {
	Client
	path       string
	negotiated bool

	wantCtx context.Context

	loadErr  error
	loadBody io.ReadCloser

	saveErr  error
	saveBody io.ReadCloser

	inspectErr  error
	inspectResp api.InspectResponse
	inspectBody []byte

	tagErr error
}

func (m *MockClient) NegotiateAPIVersion(_ context.Context) {
	m.negotiated = true
}

func (m *MockClient) ImageSave(_ context.Context, _ []string, _ ...client.ImageSaveOption) (io.ReadCloser, error) {
	if !m.negotiated {
		return nil, errors.New("you forgot to call NegotiateAPIVersion before calling ImageSave")
	}

	if m.path != "" {
		return os.Open(m.path)
	}

	return m.saveBody, m.saveErr
}

func (m *MockClient) ImageInspectWithRaw(_ context.Context, _ string) (api.InspectResponse, []byte, error) {
	return m.inspectResp, m.inspectBody, m.inspectErr
}

func (m *MockClient) ImageHistory(_ context.Context, _ string, _ ...client.ImageHistoryOption) ([]api.HistoryResponseItem, error) {
	return []api.HistoryResponseItem{
		{
			CreatedBy: "bazel build ...",
			ID:        "sha256:6e0b05049ed9c17d02e1a55e80d6599dbfcce7f4f4b022e3c673e685789c470e",
			Size:      8,
			Tags: []string{
				"bazel/v1/tarball:test_image_1",
			},
		},
	}, nil
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
			path:        imagePath,
			inspectResp: inspectResp,
		},
	}, {
		name: "save err",
		client: &MockClient{
			saveBody:    io.NopCloser(strings.NewReader("Loaded")),
			saveErr:     fmt.Errorf("locked and loaded"),
			inspectResp: inspectResp,
		},
		wantErr: "locked and loaded",
	}, {
		name: "read err",
		client: &MockClient{
			inspectResp: inspectResp,
			saveBody:    io.NopCloser(&errReader{fmt.Errorf("goodbye, world")}),
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

			err = compare.Images(img, dmn)
			if err != nil {
				if tc.wantErr == "" {
					t.Errorf("compare.Images: %v", err)
				} else if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("wanted %s to contain %s", err.Error(), tc.wantErr)
				}
			}

			err = validate.Image(dmn)
			if err != nil {
				if tc.wantErr == "" {
					t.Errorf("validate.Image: %v", err)
				} else if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("wanted %s to contain %s", err.Error(), tc.wantErr)
				}
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

	if _, err := Image(name.MustParseReference("unused")); !errors.Is(err, wantErr) {
		t.Errorf("Image(): want %v; got %v", wantErr, err)
	}
}
