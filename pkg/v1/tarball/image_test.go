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

package tarball

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

func TestManifestAndConfig(t *testing.T) {
	img, err := ImageFromPath("testdata/test_image_1.tar", nil)
	if err != nil {
		t.Fatalf("Error loading image: %v", err)
	}
	manifest, err := img.Manifest()
	if err != nil {
		t.Fatalf("Error loading manifest: %v", err)
	}
	if len(manifest.Layers) != 1 {
		t.Fatalf("layers should be 1, got %d", len(manifest.Layers))
	}

	config, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("Error loading config file: %v", err)
	}
	if len(config.History) != 1 {
		t.Fatalf("history length should be 1, got %d", len(config.History))
	}

	if err := validate.Image(img); err != nil {
		t.Errorf("Validate() = %v", err)
	}
}

func TestNullManifest(t *testing.T) {
	img, err := ImageFromPath("testdata/null_manifest.tar", nil)
	if err == nil {
		t.Fatalf("Error expected loading null image: %v", img)
	}
}

func TestNoManifest(t *testing.T) {
	img, err := ImageFromPath("testdata/no_manifest.tar", nil)
	if err == nil {
		t.Fatalf("Error expected loading image: %v", img)
	}
}

func TestBundleSingle(t *testing.T) {
	img, err := ImageFromPath("testdata/test_bundle.tar", nil)
	if err == nil {
		t.Fatalf("Error expected loading image: %v", img)
	}
}

func TestDocker25(t *testing.T) {
	img, err := ImageFromPath("testdata/hello-world-v25.tar", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Image(img); err != nil {
		t.Fatal(err)
	}
}

func TestBundleMultiple(t *testing.T) {
	for _, imgName := range []string{
		"test_image_1",
		"test_image_2",
		"test_image_1:latest",
		"test_image_2:latest",
		"index.docker.io/library/test_image_1:latest",
	} {
		t.Run(imgName, func(t *testing.T) {
			tag, err := name.NewTag(imgName, name.WeakValidation)
			if err != nil {
				t.Fatalf("Error creating tag: %v", err)
			}
			img, err := ImageFromPath("testdata/test_bundle.tar", &tag)
			if err != nil {
				t.Fatalf("Error loading image: %v", err)
			}
			if _, err := img.Manifest(); err != nil {
				t.Fatalf("Unexpected error loading manifest: %v", err)
			}

			if err := validate.Image(img, validate.Fast); err != nil {
				t.Errorf("Validate() = %v", err)
			}
		})
	}
}

func TestLayerLink(t *testing.T) {
	tag, err := name.NewTag("bazel/v1/tarball:test_image_3", name.WeakValidation)
	if err != nil {
		t.Fatalf("Error creating tag: %v", err)
	}
	img, err := ImageFromPath("testdata/test_link.tar", &tag)
	if err != nil {
		t.Fatalf("Error loading image: %v", img)
	}
	hash := v1.Hash{
		Algorithm: "sha256",
		Hex:       "8897395fd26dc44ad0e2a834335b33198cb41ac4d98dfddf58eced3853fa7b17",
	}
	layer, err := img.LayerByDiffID(hash)
	if err != nil {
		t.Fatalf("Error getting layer by diff ID: %v, %v", hash, err)
	}
	rc, err := layer.Uncompressed()
	if err != nil {
		t.Fatal(err)
	}
	bs, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) == 0 {
		t.Errorf("layer.Uncompressed() returned a link file")
	}
}

func TestSymlinkCycle(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "manifest.json",
		Linkname: "config.json",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "config.json",
		Linkname: "manifest.json",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	content := buf.Bytes()
	opener := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(content)), nil
	}

	_, err := LoadManifest(opener)
	if err == nil {
		t.Fatal("expected error for symlink cycle, got nil")
	}
	if !strings.Contains(err.Error(), "link cycle") {
		t.Fatalf("expected link cycle error, got: %v", err)
	}
}

func TestLoadManifest(t *testing.T) {
	manifest, err := LoadManifest(pathOpener("testdata/test_load_manifest.tar"))
	if err != nil {
		t.Fatalf("Error load manifest: %v", err)
	}
	if len(manifest) == 0 {
		t.Fatalf("get nothing")
	}
}

// TestManifestEmptyDiffIDs verifies Manifest() returns an error instead of
// panicking when the config has no rootfs.diff_ids. See #2192.
func TestManifestEmptyDiffIDs(t *testing.T) {
	configBytes := []byte(`{"architecture":"amd64","os":"linux","rootfs":{"type":"layers","diff_ids":[]}}`)
	// Layer payload must be gzip-detectable so Image() routes to compressedImage,
	// the path #2192 panicked on.
	var lb bytes.Buffer
	gw := gzip.NewWriter(&lb)
	if _, err := gw.Write([]byte("not-really-a-layer")); err != nil {
		t.Fatalf("gzip Write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	layerBytes := lb.Bytes()
	manifestBytes := []byte(`[{"Config":"config.json","RepoTags":["foo:latest"],"Layers":["layer.tar.gz"]}]`)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range []struct {
		name string
		data []byte
	}{
		{"config.json", configBytes},
		{"layer.tar.gz", layerBytes},
		{"manifest.json", manifestBytes},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Mode: 0644, Size: int64(len(e.data))}); err != nil {
			t.Fatalf("WriteHeader(%q): %v", e.name, err)
		}
		if _, err := tw.Write(e.data); err != nil {
			t.Fatalf("Write(%q): %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}

	tarBytes := buf.Bytes()
	img, err := Image(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBytes)), nil
	}, nil)
	if err != nil {
		t.Fatalf("Image(): %v", err)
	}

	_, err = img.Manifest()
	if err == nil {
		t.Fatal("img.Manifest(): expected error for config with empty diff_ids, got nil")
	}
	if !strings.Contains(err.Error(), "diff_ids") {
		t.Errorf("img.Manifest(): expected error mentioning diff_ids, got: %v", err)
	}
}

func TestLoadManifestPrefixPath(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	manifestBytes := []byte(`[]`)
	if err := tw.WriteHeader(&tar.Header{
		Name: "./manifest.json",
		Mode: 0644,
		Size: int64(len(manifestBytes)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(manifestBytes); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	content := buf.Bytes()
	opener := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(content)), nil
	}

	manifest, err := LoadManifest(opener)
	if err != nil {
		t.Fatalf("LoadManifest() failed: %v", err)
	}
	if len(manifest) != 0 {
		t.Errorf("expected empty manifest, got %v", manifest)
	}
}

// TestLayerByDiffIDMoreDiffIDsThanLayers verifies LayerByDiffID returns an
// error instead of panicking when the config declares more rootfs.diff_ids
// than the tarball manifest lists layers. This is the uncompressed twin of
// the compressedImage.Manifest() case fixed in #2304.
func TestLayerByDiffIDMoreDiffIDsThanLayers(t *testing.T) {
	configBytes := []byte(`{"architecture":"amd64","os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:0000000000000000000000000000000000000000000000000000000000000000"]}}`)
	// No layers in the manifest routes Image() to uncompressedImage.
	manifestBytes := []byte(`[{"Config":"config.json","RepoTags":["foo:latest"],"Layers":[]}]`)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range []struct {
		name string
		data []byte
	}{
		{"config.json", configBytes},
		{"manifest.json", manifestBytes},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Mode: 0644, Size: int64(len(e.data))}); err != nil {
			t.Fatalf("WriteHeader(%q): %v", e.name, err)
		}
		if _, err := tw.Write(e.data); err != nil {
			t.Fatalf("Write(%q): %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}

	tarBytes := buf.Bytes()
	img, err := Image(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBytes)), nil
	}, nil)
	if err != nil {
		t.Fatalf("Image(): %v", err)
	}

	if _, err := img.Layers(); err == nil {
		t.Fatal("img.Layers(): expected error for config with more diff_ids than layers, got nil")
	} else if !strings.Contains(err.Error(), "diff_id") {
		t.Errorf("img.Layers(): expected error mentioning diff_id, got: %v", err)
	}
}

func extractTarToDir(t *testing.T, tarPath, destDir string) {
	t.Helper()
	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatalf("Open(%q): %v", tarPath, err)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		target := filepath.Join(destDir, filepath.FromSlash(hdr.Name))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				t.Fatalf("MkdirAll(%q): %v", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				t.Fatalf("MkdirAll(%q): %v", filepath.Dir(target), err)
			}
			out, err := os.Create(target)
			if err != nil {
				t.Fatalf("Create(%q): %v", target, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				t.Fatalf("Copy(%q): %v", target, err)
			}
			if err := out.Close(); err != nil {
				t.Fatalf("Close(%q): %v", target, err)
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				t.Fatalf("MkdirAll(%q): %v", filepath.Dir(target), err)
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				t.Fatalf("Symlink(%q, %q): %v", hdr.Linkname, target, err)
			}
		}
	}
}

func extractTarToMapFS(t *testing.T, tarPath string) fstest.MapFS {
	t.Helper()
	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatalf("Open(%q): %v", tarPath, err)
	}
	defer f.Close()

	m := fstest.MapFS{}
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		if hdr.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("ReadAll(%q): %v", hdr.Name, err)
			}
			cleanName := path.Clean(hdr.Name)
			m[cleanName] = &fstest.MapFile{
				Data:    data,
				Mode:    hdr.FileInfo().Mode(),
				ModTime: hdr.ModTime,
			}
		}
	}
	return m
}

func TestImageFromFS_Uncompressed(t *testing.T) {
	// Test loading an uncompressed image from an unpacked directory via os.DirFS.
	tmpDir := t.TempDir()
	extractTarToDir(t, "testdata/test_image_1.tar", tmpDir)

	fsys := os.DirFS(tmpDir)
	gotImg, err := ImageFromFS(fsys, nil)
	if err != nil {
		t.Fatalf("ImageFromFS(): %v", err)
	}

	if err := validate.Image(gotImg); err != nil {
		t.Errorf("validate.Image(): %v", err)
	}

	wantImg, err := ImageFromPath("testdata/test_image_1.tar", nil)
	if err != nil {
		t.Fatalf("ImageFromPath(): %v", err)
	}

	wantManifest, err := wantImg.Manifest()
	if err != nil {
		t.Fatalf("wantImg.Manifest(): %v", err)
	}
	gotManifest, err := gotImg.Manifest()
	if err != nil {
		t.Fatalf("gotImg.Manifest(): %v", err)
	}
	if wantManifest.Config.Digest != gotManifest.Config.Digest {
		t.Errorf("Config digest mismatch: want %v, got %v", wantManifest.Config.Digest, gotManifest.Config.Digest)
	}
	if len(wantManifest.Layers) != len(gotManifest.Layers) {
		t.Fatalf("Layer count mismatch: want %d, got %d", len(wantManifest.Layers), len(gotManifest.Layers))
	}
	for i := range wantManifest.Layers {
		if wantManifest.Layers[i].Digest != gotManifest.Layers[i].Digest {
			t.Errorf("Layer %d digest mismatch: want %v, got %v", i, wantManifest.Layers[i].Digest, gotManifest.Layers[i].Digest)
		}
	}
}

func TestImageFromFS_Docker25(t *testing.T) {
	// hello-world-v25.tar has compressed layers.
	mfs := extractTarToMapFS(t, "testdata/hello-world-v25.tar")

	gotImg, err := ImageFromFS(mfs, nil)
	if err != nil {
		t.Fatalf("ImageFromFS(): %v", err)
	}

	if err := validate.Image(gotImg); err != nil {
		t.Errorf("validate.Image(): %v", err)
	}

	wantImg, err := ImageFromPath("testdata/hello-world-v25.tar", nil)
	if err != nil {
		t.Fatalf("ImageFromPath(): %v", err)
	}

	wantDigest, err := wantImg.Digest()
	if err != nil {
		t.Fatalf("wantImg.Digest(): %v", err)
	}
	gotDigest, err := gotImg.Digest()
	if err != nil {
		t.Fatalf("gotImg.Digest(): %v", err)
	}
	if wantDigest != gotDigest {
		t.Errorf("Digest mismatch: want %v, got %v", wantDigest, gotDigest)
	}

	// Loading a nonexistent tag should fail.
	nonexistentTag, err := name.NewTag("gcr.io/test/nonexistent:latest")
	if err != nil {
		t.Fatalf("name.NewTag: %v", err)
	}
	if _, err := ImageFromFS(mfs, &nonexistentTag); err == nil {
		t.Errorf("expected error for nonexistent tag, got nil")
	}
}

func TestImageFromFS_SymlinkLayer(t *testing.T) {
	// Test loading an image whose layers are symlinks via os.DirFS.
	tmpDir := t.TempDir()
	extractTarToDir(t, "testdata/test_link.tar", tmpDir)

	tag, err := name.NewTag("bazel/v1/tarball:test_image_3", name.WeakValidation)
	if err != nil {
		t.Fatalf("name.NewTag: %v", err)
	}

	fsys := os.DirFS(tmpDir)
	gotImg, err := ImageFromFS(fsys, &tag)
	if err != nil {
		t.Fatalf("ImageFromFS(): %v", err)
	}

	hash := v1.Hash{
		Algorithm: "sha256",
		Hex:       "8897395fd26dc44ad0e2a834335b33198cb41ac4d98dfddf58eced3853fa7b17",
	}
	layer, err := gotImg.LayerByDiffID(hash)
	if err != nil {
		t.Fatalf("LayerByDiffID: %v", err)
	}
	rc, err := layer.Uncompressed()
	if err != nil {
		t.Fatalf("layer.Uncompressed(): %v", err)
	}
	defer rc.Close()
	bs, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("io.ReadAll: %v", err)
	}
	if len(bs) == 0 {
		t.Errorf("layer.Uncompressed() returned empty content")
	}
}
