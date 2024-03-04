// Copyright 2019 The original author or authors
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

package layout

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

func mustOCILayout(t *testing.T) Path {
	tmp := t.TempDir()
	os.WriteFile(fmt.Sprintf("%s/index.json", tmp), []byte("{}"), os.ModePerm)
	path, err := FromPath(tmp)
	if err != nil {
		t.Errorf("FromPath() = %v", err)
	}
	return path
}

func mustHaveManifest(t *testing.T, p Path, ref string) {
	idx, err := p.ImageIndex()
	if err != nil {
		t.Errorf("path.ImageIndex() = %v", err)
	}
	manifest, err := idx.IndexManifest()
	if err != nil {
		t.Errorf("idx.IndexManifest() = %v", err)
	}
	if manifest.Manifests[0].Annotations["org.opencontainers.image.ref.name"] != ref {
		t.Errorf("image does not contain %s", ref)
	}
}

func enumerateImageBlobs(t *testing.T, img v1.Image) []v1.Hash {
	mgd, err := img.Digest()
	if err != nil {
		t.Errorf("img.Digest() = %v", err)
	}
	cdg, err := img.ConfigName()
	if err != nil {
		t.Errorf("img.ConfigName() = %v", err)
	}
	blobs := []v1.Hash{
		mgd,
		cdg,
	}
	layers, err := img.Layers()
	if err != nil {
		t.Errorf("img.Layers() = %v", err)
	}
	for _, layer := range layers {
		ldg, err := layer.Digest()
		if err != nil {
			t.Errorf("layer.Digest() = %v", err)
		}
		blobs = append(blobs, ldg)
	}
	return blobs
}

func enumerateImageIndexBlobs(t *testing.T, idx v1.ImageIndex) []v1.Hash {
	mgd, err := idx.Digest()
	if err != nil {
		t.Errorf("idx.Digest() = %v", err)
	}
	blobs := []v1.Hash{mgd}
	imf, err := idx.IndexManifest()
	if err != nil {
		t.Errorf("img.Layers() = %v", err)
	}
	for _, manifest := range imf.Manifests {
		im, err := idx.Image(manifest.Digest)
		if err != nil {
			t.Errorf("idx.Image = %v", err)
		}
		blobs = append(blobs, enumerateImageBlobs(t, im)...)
	}
	return blobs
}

func mustHaveBlobs(t *testing.T, p Path, blobs []v1.Hash) {
	for _, blob := range blobs {
		if !p.BlobExists(blob) {
			t.Fatalf("blob %s/%s is missing", blob.Algorithm, blob.Hex)
		}
	}
}

func TestCanPushRandomImage(t *testing.T) {
	img, err := random.Image(1024, 10)
	if err != nil {
		t.Fatalf("random.Image() = %v", err)
	}
	path := mustOCILayout(t)
	pusher := NewPusher(path)
	ref := name.MustParseReference("local.random/image:latest")
	err = pusher.Push(context.TODO(), ref, img)
	if err != nil {
		t.Errorf("pusher.Push() = %v", err)
	}
	mustHaveManifest(t, path, "local.random/image:latest")
	mustHaveBlobs(t, path, enumerateImageBlobs(t, img))
}

func TestCanPushRandomImageIndex(t *testing.T) {
	idx, err := random.Index(1024, 3, 3)
	if err != nil {
		t.Fatalf("random.Index() = %v", err)
	}
	path := mustOCILayout(t)
	pusher := NewPusher(path)
	ref := name.MustParseReference("local.random/index:latest")
	err = pusher.Push(context.TODO(), ref, idx)
	if err != nil {
		t.Errorf("pusher.Push() = %v", err)
	}
	mustHaveManifest(t, path, "local.random/index:latest")
	mustHaveBlobs(t, path, enumerateImageIndexBlobs(t, idx))
}

func TestCanPushImageIndex(t *testing.T) {
	lp, err := FromPath(testPath)
	if err != nil {
		t.Fatalf("FromPath() = %v", err)
	}
	img, err := lp.Image(manifestDigest)
	if err != nil {
		t.Fatalf("Image() = %v", err)
	}

	path := mustOCILayout(t)
	pusher := NewPusher(path)
	ref := name.MustParseReference("local/index:latest")

	err = pusher.Push(context.TODO(), ref, img)
	if err != nil {
		t.Errorf("pusher.Push() = %v", err)
	}
	mustHaveManifest(t, path, "local/index:latest")
	mustHaveBlobs(t, path, enumerateImageBlobs(t, img))
}

func TestCanPushImage(t *testing.T) {
	lp, err := FromPath(testPath)
	if err != nil {
		t.Fatalf("FromPath() = %v", err)
	}
	img, err := lp.Image(manifestDigest)
	if err != nil {
		t.Fatalf("Image() = %v", err)
	}

	path := mustOCILayout(t)
	pusher := NewPusher(path)
	ref := name.MustParseReference("local/index:latest")

	err = pusher.Push(context.TODO(), ref, img)
	if err != nil {
		t.Errorf("pusher.Push() = %v", err)
	}
	mustHaveManifest(t, path, "local/index:latest")
	mustHaveBlobs(t, path, enumerateImageBlobs(t, img))
}
