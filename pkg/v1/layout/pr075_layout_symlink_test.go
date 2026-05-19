package layout_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestLayoutBlobSymlinkRejected(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "pr075-layout-secret.txt")
	secret := []byte("PR075_LAYOUT_SECRET\n")
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
	configHash, _, err := v1.SHA256(strings.NewReader(string(config)))
	if err != nil {
		t.Fatalf("SHA256(config): %v", err)
	}
	if err := os.WriteFile(filepath.Join(blobDir, configHash.Hex), config, 0o644); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}

	layerHash := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("a", 64)}
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
	manifestHash, _, err := v1.SHA256(strings.NewReader(string(manifestBytes)))
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

	idx, err := layout.ImageIndexFromPath(layoutDir)
	if err != nil {
		t.Fatalf("ImageIndexFromPath: %v", err)
	}
	img, err := idx.Image(manifestHash)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}
	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}
	rc, err := layers[0].Compressed()
	if err == nil {
		rc.Close()
		t.Fatalf("Compressed unexpectedly opened symlink blob")
	}

	t.Logf("PR075_FIXED_LAYOUT_BLOB_SYMLINK_REJECTED layout=%q outside=%q digest=%s err=%v", layoutDir, outside, layerHash, err)
}

func TestLayoutBlobSymlinkNotUploadedByRemoteWrite(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "pr075-layout-upload-secret.txt")
	secret := []byte("PR075_LAYOUT_UPLOAD_SECRET\n")
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
	configHash, _, err := v1.SHA256(strings.NewReader(string(config)))
	if err != nil {
		t.Fatalf("SHA256(config): %v", err)
	}
	if err := os.WriteFile(filepath.Join(blobDir, configHash.Hex), config, 0o644); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}

	layerHash := v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("b", 64)}
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
	manifestHash, _, err := v1.SHA256(strings.NewReader(string(manifestBytes)))
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

	idx, err := layout.ImageIndexFromPath(layoutDir)
	if err != nil {
		t.Fatalf("ImageIndexFromPath: %v", err)
	}
	img, err := idx.Image(manifestHash)
	if err != nil {
		t.Fatalf("Image: %v", err)
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

	ref, err := name.NewTag(strings.TrimPrefix(server.URL, "http://")+"/repo:tag", name.WeakValidation, name.Insecure)
	if err != nil {
		t.Fatalf("NewTag: %v", err)
	}
	err = remote.Write(ref, img, remote.WithJobs(1))
	if err == nil {
		t.Fatalf("remote.Write unexpectedly succeeded with symlink blob")
	}

	for _, body := range uploaded {
		if string(body) == string(secret) {
			t.Fatalf("outside secret was uploaded despite symlink rejection")
		}
	}
	t.Logf("PR075_FIXED_LAYOUT_BLOB_SYMLINK_REMOTE_WRITE_REJECTED outside=%q uploaded_blobs=%d err=%v", outside, len(uploaded), err)
}
