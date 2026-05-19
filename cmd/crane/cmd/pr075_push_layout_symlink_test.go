package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestCranePushLayoutSymlinkBlobNotUploaded(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "pr075-crane-push-layout-secret.txt")
	secret := []byte("PR075_CRANE_PUSH_LAYOUT_SECRET\n")
	if err := os.WriteFile(outside, secret, 0o600); err != nil {
		t.Fatalf("WriteFile(outside): %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	layoutDir := filepath.Join(root, "layout")
	blobDir := filepath.Join(layoutDir, "blobs", "sha256")
	if err := os.MkdirAll(blobDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(blobDir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(layoutDir, "oci-layout"), []byte(`{"imageLayoutVersion":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(oci-layout): %v", err)
	}

	config := []byte(`{"architecture":"amd64","os":"linux","rootfs":{"type":"layers","diff_ids":[]}}`)
	configHash, _, err := v1.SHA256(bytes.NewReader(config))
	if err != nil {
		t.Fatalf("SHA256(config): %v", err)
	}
	if err := os.WriteFile(filepath.Join(blobDir, configHash.Hex), config, 0o644); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}

	layerHash := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("c", 64)}
	if err := os.Symlink(outside, filepath.Join(blobDir, layerHash.Hex)); err != nil {
		t.Skipf("Symlink unavailable: %v", err)
	}

	manifest := v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		Config: v1.Descriptor{
			MediaType: types.OCIConfigJSON,
			Size:      int64(len(config)),
			Digest:    configHash,
		},
		Layers: []v1.Descriptor{{
			MediaType: types.OCILayer,
			Size:      int64(len(secret)),
			Digest:    layerHash,
		}},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal(manifest): %v", err)
	}
	manifestHash, _, err := v1.SHA256(bytes.NewReader(manifestBytes))
	if err != nil {
		t.Fatalf("SHA256(manifest): %v", err)
	}
	if err := os.WriteFile(filepath.Join(blobDir, manifestHash.Hex), manifestBytes, 0o644); err != nil {
		t.Fatalf("WriteFile(manifest): %v", err)
	}

	index := v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     types.OCIImageIndex,
		Manifests: []v1.Descriptor{{
			MediaType: types.OCIManifestSchema1,
			Size:      int64(len(manifestBytes)),
			Digest:    manifestHash,
		}},
	}
	indexBytes, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("Marshal(index): %v", err)
	}
	if err := os.WriteFile(filepath.Join(layoutDir, "index.json"), indexBytes, 0o644); err != nil {
		t.Fatalf("WriteFile(index): %v", err)
	}

	var mu sync.Mutex
	var uploaded [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if r.URL.Path == "/v2/" {
				w.WriteHeader(http.StatusOK)
				return
			}
		case http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
			return
		case http.MethodPost:
			w.Header().Set("Location", "/v2/repo/blobs/uploads/pr075")
			w.WriteHeader(http.StatusAccepted)
			return
		case http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("ReadAll(PATCH): %v", err)
			}
			mu.Lock()
			uploaded = append(uploaded, body)
			mu.Unlock()
			w.Header().Set("Location", "/v2/repo/blobs/uploads/pr075")
			w.WriteHeader(http.StatusAccepted)
			return
		case http.MethodPut:
			w.WriteHeader(http.StatusCreated)
			return
		}
		t.Errorf("unexpected registry request: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cmd := New("crane", "test", nil)
	cmd.SetArgs([]string{"--insecure", "push", layoutDir, fmt.Sprintf("%s/repo:tag", strings.TrimPrefix(server.URL, "http://"))})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	err = cmd.Execute()
	if err == nil {
		t.Fatalf("crane push unexpectedly succeeded with symlink blob")
	}

	for _, body := range uploaded {
		if bytes.Equal(body, secret) {
			t.Fatalf("outside secret was uploaded despite symlink rejection")
		}
	}
	t.Logf("PR075_FIXED_CRANE_PUSH_LAYOUT_SYMLINK_BLOB_NOT_UPLOADED outside=%q uploaded_blobs=%d err=%v", outside, len(uploaded), err)
}

func TestCraneIndexAppendLocalLayoutSymlinkTargetNotCopied(t *testing.T) {
	layoutDir, outside, secret, layerHash := writePR075CraneLayoutWithSymlinkBlob(t, "e")
	outDir := filepath.Join(t.TempDir(), "out-layout")

	cmd := New("crane", "test", nil)
	cmd.SetArgs([]string{"index", "append", outDir, "-m", layoutDir})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("crane index append unexpectedly succeeded with symlink blob")
	}

	got, readErr := os.ReadFile(filepath.Join(outDir, "blobs", layerHash.Algorithm, layerHash.Hex))
	if readErr == nil && bytes.Equal(got, secret) {
		t.Fatalf("outside secret was copied despite symlink rejection")
	}
	t.Logf("PR075_FIXED_CRANE_INDEX_APPEND_LOCAL_LAYOUT_SYMLINK_TARGET_NOT_COPIED outside=%q output_layout=%q err=%v", outside, outDir, err)
}

func writePR075CraneLayoutWithSymlinkBlob(t *testing.T, hexByte string) (layoutDir, outside string, secret []byte, layerHash v1.Hash) {
	t.Helper()
	root := t.TempDir()
	outside = filepath.Join(filepath.Dir(root), "pr075-crane-layout-secret-"+hexByte+".txt")
	secret = []byte("PR075_CRANE_LAYOUT_SECRET_" + hexByte + "\n")
	if err := os.WriteFile(outside, secret, 0o600); err != nil {
		t.Fatalf("WriteFile(outside): %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	layoutDir = filepath.Join(root, "layout")
	blobDir := filepath.Join(layoutDir, "blobs", "sha256")
	if err := os.MkdirAll(blobDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(blobDir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(layoutDir, "oci-layout"), []byte(`{"imageLayoutVersion":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(oci-layout): %v", err)
	}

	config := []byte(`{"architecture":"amd64","os":"linux","rootfs":{"type":"layers","diff_ids":[]}}`)
	configHash, _, err := v1.SHA256(bytes.NewReader(config))
	if err != nil {
		t.Fatalf("SHA256(config): %v", err)
	}
	if err := os.WriteFile(filepath.Join(blobDir, configHash.Hex), config, 0o644); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}

	layerHash = v1.Hash{Algorithm: "sha256", Hex: strings.Repeat(hexByte, 64)}
	if err := os.Symlink(outside, filepath.Join(blobDir, layerHash.Hex)); err != nil {
		t.Skipf("Symlink unavailable: %v", err)
	}

	manifest := v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		Config: v1.Descriptor{
			MediaType: types.OCIConfigJSON,
			Size:      int64(len(config)),
			Digest:    configHash,
		},
		Layers: []v1.Descriptor{{
			MediaType: types.OCILayer,
			Size:      int64(len(secret)),
			Digest:    layerHash,
		}},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal(manifest): %v", err)
	}
	manifestHash, _, err := v1.SHA256(bytes.NewReader(manifestBytes))
	if err != nil {
		t.Fatalf("SHA256(manifest): %v", err)
	}
	if err := os.WriteFile(filepath.Join(blobDir, manifestHash.Hex), manifestBytes, 0o644); err != nil {
		t.Fatalf("WriteFile(manifest): %v", err)
	}

	index := v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     types.OCIImageIndex,
		Manifests: []v1.Descriptor{{
			MediaType: types.OCIManifestSchema1,
			Size:      int64(len(manifestBytes)),
			Digest:    manifestHash,
		}},
	}
	indexBytes, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("Marshal(index): %v", err)
	}
	if err := os.WriteFile(filepath.Join(layoutDir, "index.json"), indexBytes, 0o644); err != nil {
		t.Fatalf("WriteFile(index): %v", err)
	}
	return layoutDir, outside, secret, layerHash
}
