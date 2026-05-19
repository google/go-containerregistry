package layout_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func TestLayoutAppendImageDoesNotCopySymlinkTargetOutsideFile(t *testing.T) {
	layoutDir, outside, secret, layerHash, manifestHash := writePR075AppendLayoutWithSymlinkBlob(t)

	idx, err := layout.ImageIndexFromPath(layoutDir)
	if err != nil {
		t.Fatalf("ImageIndexFromPath: %v", err)
	}
	img, err := idx.Image(manifestHash)
	if err != nil {
		t.Fatalf("Image: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "out-layout")
	out, err := layout.Write(outDir, empty.Index)
	if err != nil {
		t.Fatalf("layout.Write(out): %v", err)
	}
	if err := out.AppendImage(img); err == nil {
		t.Fatalf("AppendImage unexpectedly succeeded with symlink blob")
	}

	got, err := os.ReadFile(filepath.Join(outDir, "blobs", layerHash.Algorithm, layerHash.Hex))
	if err == nil && string(got) == string(secret) {
		t.Fatalf("outside secret was copied into output layout despite symlink rejection")
	}
	t.Logf("PR075_FIXED_LAYOUT_APPENDIMAGE_SYMLINK_TARGET_NOT_COPIED outside=%q output_blob=%s", outside, layerHash)
}

func writePR075AppendLayoutWithSymlinkBlob(t *testing.T) (layoutDir, outside string, secret []byte, layerHash, manifestHash v1.Hash) {
	t.Helper()
	root := t.TempDir()
	outside = filepath.Join(filepath.Dir(root), "pr075-layout-append-secret.txt")
	secret = []byte("PR075_LAYOUT_APPEND_SECRET\n")
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
	configHash, _, err := v1.SHA256(strings.NewReader(string(config)))
	if err != nil {
		t.Fatalf("SHA256(config): %v", err)
	}
	if err := os.WriteFile(filepath.Join(blobDir, configHash.Hex), config, 0o644); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}

	layerHash = v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("d", 64)}
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
	manifestHash, _, err = v1.SHA256(strings.NewReader(string(manifestBytes)))
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
	return layoutDir, outside, secret, layerHash, manifestHash
}
