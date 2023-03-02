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

package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

const bogusDigest = "sha256:deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

type withDigest interface {
	Digest() (v1.Hash, error)
}

func mustDigest(t *testing.T, img withDigest) v1.Hash {
	h, err := img.Digest()
	if err != nil {
		t.Fatalf("Digest() = %v", err)
	}
	return h
}

func mustManifest(t *testing.T, img v1.Image) *v1.Manifest {
	m, err := img.Manifest()
	if err != nil {
		t.Fatalf("Manifest() = %v", err)
	}
	return m
}

func mustRawManifest(t *testing.T, img Taggable) []byte {
	m, err := img.RawManifest()
	if err != nil {
		t.Fatalf("RawManifest() = %v", err)
	}
	return m
}

func mustRawConfigFile(t *testing.T, img v1.Image) []byte {
	c, err := img.RawConfigFile()
	if err != nil {
		t.Fatalf("RawConfigFile() = %v", err)
	}
	return c
}

func randomImage(t *testing.T) v1.Image {
	rnd, err := random.Image(1024, 1)
	if err != nil {
		t.Fatalf("random.Image() = %v", err)
	}
	return rnd
}

func newReference(host, repo, ref string) (name.Reference, error) {
	tag, err := name.NewTag(fmt.Sprintf("%s/%s:%s", host, repo, ref), name.WeakValidation)
	if err == nil {
		return tag, nil
	}
	return name.NewDigest(fmt.Sprintf("%s/%s@%s", host, repo, ref), name.WeakValidation)
}

// TODO(jonjohnsonjr): Make this real.
func TestMediaType(t *testing.T) {
	img := remoteImage{}
	got, err := img.MediaType()
	if err != nil {
		t.Fatalf("MediaType() = %v", err)
	}
	want := types.DockerManifestSchema2
	if got != want {
		t.Errorf("MediaType() = %v, want %v", got, want)
	}
}

func TestRawManifestDigests(t *testing.T) {
	img := randomImage(t)
	expectedRepo := "foo/bar"

	cases := []struct {
		name          string
		ref           string
		responseBody  []byte
		contentDigest string
		wantErr       bool
	}{{
		name:          "normal pull, by tag",
		ref:           "latest",
		responseBody:  mustRawManifest(t, img),
		contentDigest: mustDigest(t, img).String(),
		wantErr:       false,
	}, {
		name:          "normal pull, by digest",
		ref:           mustDigest(t, img).String(),
		responseBody:  mustRawManifest(t, img),
		contentDigest: mustDigest(t, img).String(),
		wantErr:       false,
	}, {
		name:          "right content-digest, wrong body, by digest",
		ref:           mustDigest(t, img).String(),
		responseBody:  []byte("not even json"),
		contentDigest: mustDigest(t, img).String(),
		wantErr:       true,
	}, {
		name:          "right body, wrong content-digest, by tag",
		ref:           "latest",
		responseBody:  mustRawManifest(t, img),
		contentDigest: bogusDigest,
		wantErr:       false,
	}, {
		// NB: This succeeds! We don't care what the registry thinks.
		name:          "right body, wrong content-digest, by digest",
		ref:           mustDigest(t, img).String(),
		responseBody:  mustRawManifest(t, img),
		contentDigest: bogusDigest,
		wantErr:       false,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifestPath := fmt.Sprintf("/v2/%s/manifests/%s", expectedRepo, tc.ref)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case manifestPath:
					if r.Method != http.MethodGet {
						t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
					}

					w.Header().Set("Docker-Content-Digest", tc.contentDigest)
					w.Write(tc.responseBody)
				default:
					t.Fatalf("Unexpected path: %v", r.URL.Path)
				}
			}))
			defer server.Close()
			u, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("url.Parse(%v) = %v", server.URL, err)
			}

			ref, err := newReference(u.Host, expectedRepo, tc.ref)
			if err != nil {
				t.Fatalf("url.Parse(%v, %v, %v) = %v", u.Host, expectedRepo, tc.ref, err)
			}

			rmt := remoteImage{
				fetcher: fetcher{
					Ref:     ref,
					Client:  http.DefaultClient,
					context: context.Background(),
				},
			}

			if _, err := rmt.RawManifest(); (err != nil) != tc.wantErr {
				t.Errorf("RawManifest() wrong error: %v, want %v: %v\n", (err != nil), tc.wantErr, err)
			}
		})
	}
}

func TestRawManifestNotFound(t *testing.T) {
	expectedRepo := "foo/bar"
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	img := remoteImage{
		fetcher: fetcher{
			Ref:     mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo)),
			Client:  http.DefaultClient,
			context: context.Background(),
		},
	}

	if _, err := img.RawManifest(); err == nil {
		t.Error("RawManifest() = nil; wanted error")
	}
}

func TestRawConfigFileNotFound(t *testing.T) {
	img := randomImage(t)
	expectedRepo := "foo/bar"
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)
	configPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, mustConfigName(t, img))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case configPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.WriteHeader(http.StatusNotFound)
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawManifest(t, img))
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	rmt := remoteImage{
		fetcher: fetcher{
			Ref:     mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo)),
			Client:  http.DefaultClient,
			context: context.Background(),
		},
	}

	if _, err := rmt.RawConfigFile(); err == nil {
		t.Error("RawConfigFile() = nil; wanted error")
	}
}

func TestAcceptHeaders(t *testing.T) {
	img := randomImage(t)
	expectedRepo := "foo/bar"
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			wantAccept := strings.Join([]string{
				string(types.DockerManifestSchema2),
				string(types.OCIManifestSchema1),
			}, ",")
			if got, want := r.Header.Get("Accept"), wantAccept; got != want {
				t.Errorf("Accept header; got %v, want %v", got, want)
			}
			w.Write(mustRawManifest(t, img))
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	rmt := &remoteImage{
		fetcher: fetcher{
			Ref:     mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo)),
			Client:  http.DefaultClient,
			context: context.Background(),
		},
	}
	manifest, err := rmt.RawManifest()
	if err != nil {
		t.Errorf("RawManifest() = %v", err)
	}
	if got, want := manifest, mustRawManifest(t, img); !bytes.Equal(got, want) {
		t.Errorf("RawManifest() = %v, want %v", got, want)
	}
}

func TestImage(t *testing.T) {
	img := randomImage(t)
	expectedRepo := "foo/bar"
	layerDigest := mustManifest(t, img).Layers[0].Digest
	layerSize := mustManifest(t, img).Layers[0].Size
	configPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, mustConfigName(t, img))
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)
	layerPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, layerDigest)
	manifestReqCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case configPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawConfigFile(t, img))
		case manifestPath:
			manifestReqCount++
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawManifest(t, img))
		case layerPath:
			t.Fatalf("BlobSize should not make any request: %v", r.URL.Path)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	tag := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo))
	rmt, err := Image(tag, WithTransport(http.DefaultTransport), WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		t.Errorf("Image() = %v", err)
	}

	if got, want := mustRawManifest(t, rmt), mustRawManifest(t, img); !bytes.Equal(got, want) {
		t.Errorf("RawManifest() = %v, want %v", got, want)
	}
	if got, want := mustRawConfigFile(t, rmt), mustRawConfigFile(t, img); !bytes.Equal(got, want) {
		t.Errorf("RawConfigFile() = %v, want %v", got, want)
	}
	// Make sure caching the manifest works.
	if manifestReqCount != 1 {
		t.Errorf("RawManifest made %v requests, expected 1", manifestReqCount)
	}

	l, err := rmt.LayerByDigest(layerDigest)
	if err != nil {
		t.Errorf("LayerByDigest() = %v", err)
	}
	// BlobSize should not HEAD.
	size, err := l.Size()
	if err != nil {
		t.Errorf("BlobSize() = %v", err)
	}
	if got, want := size, layerSize; want != got {
		t.Errorf("BlobSize() = %v want %v", got, want)
	}
}

func TestPullingManifestList(t *testing.T) {
	idx := randomIndex(t)
	expectedRepo := "foo/bar"
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)
	childDigest := mustIndexManifest(t, idx).Manifests[1].Digest
	child := mustChild(t, idx, childDigest)
	childPath := fmt.Sprintf("/v2/%s/manifests/%s", expectedRepo, childDigest)
	fakePlatformChildDigest := mustIndexManifest(t, idx).Manifests[0].Digest
	fakePlatformChild := mustChild(t, idx, fakePlatformChildDigest)
	fakePlatformChildPath := fmt.Sprintf("/v2/%s/manifests/%s", expectedRepo, fakePlatformChildDigest)
	configPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, mustConfigName(t, child))

	fakePlatform := v1.Platform{
		Architecture: "not-real-arch",
		OS:           "not-real-os",
	}

	// Rewrite the index to make sure the desired platform matches the second child.
	manifest, err := idx.IndexManifest()
	if err != nil {
		t.Fatal(err)
	}
	// Make sure the first manifest doesn't match.
	manifest.Manifests[0].Platform = &fakePlatform
	// Make sure the second manifest does.
	manifest.Manifests[1].Platform = &defaultPlatform
	// Do short-circuiting via Data.
	manifest.Manifests[1].Data = mustRawManifest(t, child)
	rawManifest, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Header().Set("Content-Type", string(mustMediaType(t, idx)))
			w.Write(rawManifest)
		case childPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawManifest(t, child))
		case configPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawConfigFile(t, child))
		case fakePlatformChildPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawManifest(t, fakePlatformChild))
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	tag := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo))
	rmtChild, err := Image(tag)
	if err != nil {
		t.Errorf("Image() = %v", err)
	}

	// Test that child works as expected.
	if got, want := mustRawManifest(t, rmtChild), mustRawManifest(t, child); !bytes.Equal(got, want) {
		t.Errorf("RawManifest() = %v, want %v", string(got), string(want))
	}
	if got, want := mustRawConfigFile(t, rmtChild), mustRawConfigFile(t, child); !bytes.Equal(got, want) {
		t.Errorf("RawConfigFile() = %v, want %v", got, want)
	}

	// Make sure we can roundtrip platform info via Descriptor.
	img, err := Image(tag, WithPlatform(fakePlatform))
	if err != nil {
		t.Fatal(err)
	}
	desc, err := partial.Descriptor(img)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(*desc.Platform, fakePlatform); diff != "" {
		t.Errorf("Desciptor() (-want +got) = %v", diff)
	}
}

func TestPullingManifestListNoMatch(t *testing.T) {
	idx := randomIndex(t)
	expectedRepo := "foo/bar"
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)
	childDigest := mustIndexManifest(t, idx).Manifests[1].Digest
	child := mustChild(t, idx, childDigest)
	childPath := fmt.Sprintf("/v2/%s/manifests/%s", expectedRepo, childDigest)
	configPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, mustConfigName(t, child))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Header().Set("Content-Type", string(mustMediaType(t, idx)))
			w.Write(mustRawManifest(t, idx))
		case childPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawManifest(t, child))
		case configPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawConfigFile(t, child))
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}
	platform := v1.Platform{
		Architecture: "not-real-arch",
		OS:           "not-real-os",
	}
	tag := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo))
	if _, err := Image(tag, WithPlatform(platform)); err == nil {
		t.Errorf("Image succeeded, wanted err")
	}
}

func TestValidate(t *testing.T) {
	img, err := random.Image(1024, 5)
	if err != nil {
		t.Fatal(err)
	}

	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	tag, err := name.NewTag(u.Host + "/foo/bar")
	if err != nil {
		t.Fatal(err)
	}

	if err := Write(tag, img); err != nil {
		t.Fatal(err)
	}

	img, err = Image(tag)
	if err != nil {
		t.Fatal(err)
	}

	if err := validate.Image(img); err != nil {
		t.Errorf("failed to validate remote.Image: %v", err)
	}
}

func TestPullingForeignLayer(t *testing.T) {
	// For that sweet, sweet coverage in options.
	var b bytes.Buffer
	logs.Debug.SetOutput(&b)

	img := randomImage(t)
	expectedRepo := "foo/bar"
	foreignPath := "/foreign/path"

	foreignLayer, err := random.Layer(1024, types.DockerForeignLayer)
	if err != nil {
		t.Fatal(err)
	}

	foreignServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case foreignPath:
			compressed, err := foreignLayer.Compressed()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(w, compressed); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer foreignServer.Close()
	fu, err := url.Parse(foreignServer.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", foreignServer.URL, err)
	}

	img, err = mutate.Append(img, mutate.Addendum{
		Layer: foreignLayer,
		URLs: []string{
			"http://" + path.Join(fu.Host, foreignPath),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set up a fake registry that will respond 404 to the foreign layer,
	// but serve everything else correctly.
	configPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, mustConfigName(t, img))
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)
	foreignLayerDigest := mustManifest(t, img).Layers[1].Digest
	foreignLayerPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, foreignLayerDigest)
	layerDigest := mustManifest(t, img).Layers[0].Digest
	layerPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, layerDigest)

	layer, err := img.LayerByDigest(layerDigest)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case configPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawConfigFile(t, img))
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawManifest(t, img))
		case layerPath:
			compressed, err := layer.Compressed()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(w, compressed); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusOK)
		case foreignLayerPath:
			// Not here!
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}

	// Pull from the registry and ensure that everything Validates; i.e. that
	// we pull the layer from the foreignServer.
	tag := mustNewTag(t, fmt.Sprintf("%s/%s:latest", u.Host, expectedRepo))
	rmt, err := Image(tag, WithTransport(http.DefaultTransport))
	if err != nil {
		t.Errorf("Image() = %v", err)
	}

	if err := validate.Image(rmt); err != nil {
		t.Errorf("failed to validate foreign image: %v", err)
	}

	// Set up a fake registry and write what we pulled to it.
	// This ensures we get coverage for the remoteLayer.MediaType path.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err = url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	dst := fmt.Sprintf("%s/test/foreign/upload", u.Host)
	ref, err := name.ParseReference(dst)
	if err != nil {
		t.Fatal(err)
	}

	if err := Write(ref, rmt); err != nil {
		t.Errorf("failed to Write: %v", err)
	}
}

func TestData(t *testing.T) {
	img := randomImage(t)
	manifest, err := img.Manifest()
	if err != nil {
		t.Fatal(err)
	}
	layers, err := img.Layers()
	if err != nil {
		t.Fatal(err)
	}
	cb, err := img.RawConfigFile()
	if err != nil {
		t.Fatal(err)
	}

	manifest.Config.Data = cb
	rc, err := layers[0].Compressed()
	if err != nil {
		t.Fatal(err)
	}
	lb, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Layers[0].Data = lb
	rawManifest, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case "/v2/test/manifests/latest":
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(rawManifest)
		default:
			// explode if we try to read blob or config
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%v) = %v", server.URL, err)
	}
	ref, err := newReference(u.Host, "test", "latest")
	if err != nil {
		t.Fatal(err)
	}
	rmt, err := Image(ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Image(rmt); err != nil {
		t.Fatal(err)
	}
}
