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

package crane

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Load reads the tarball at path as a v1.Image.
// It supports Docker tarballs and single-image OCI layout tarballs.
func Load(path string, opt ...Option) (v1.Image, error) {
	return LoadTag(path, "", opt...)
}

// LoadTag reads a tag from the tarball at path as a v1.Image.
// If tag is "", it will attempt to read the tarball as a single image.
func LoadTag(path, tag string, opt ...Option) (v1.Image, error) {
	if tag == "" {
		img, err := tarball.ImageFromPath(path, nil)
		if err == nil {
			return img, nil
		}
		ociImg, ociErr := loadOCILayoutTarball(path)
		if ociErr == nil {
			return ociImg, nil
		}
		return nil, fmt.Errorf("loading as docker tarball: %w; loading as OCI layout tarball: %v", err, ociErr)
	}

	o := makeOptions(opt...)
	t, err := name.NewTag(tag, o.Name...)
	if err != nil {
		return nil, fmt.Errorf("parsing tag %q: %w", tag, err)
	}
	return tarball.ImageFromPath(path, &t)
}

func loadOCILayoutTarball(path string) (v1.Image, error) {
	dir, err := os.MkdirTemp("", "crane-oci-layout-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	if err := extractTar(path, dir); err != nil {
		return nil, err
	}

	idx, err := layout.ImageIndexFromPath(dir)
	if err != nil {
		return nil, err
	}
	img, err := idx.Image(v1.Hash{})
	if err != nil {
		return nil, err
	}

	ref := name.MustParseReference("example.com/oci-layout:latest")
	var b bytes.Buffer
	if err := tarball.Write(ref, img, &b); err != nil {
		return nil, err
	}
	return tarball.Image(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(b.Bytes())), nil
	}, nil)
}

func extractTar(path, dir string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		name := filepath.Clean(header.Name)
		if name == "." {
			continue
		}
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("tar entry escapes destination: %q", header.Name)
		}

		target := filepath.Join(dir, name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := writeTarFile(target, tr, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			continue
		default:
			return fmt.Errorf("unsupported tar entry %q type %d", header.Name, header.Typeflag)
		}
	}
}

func writeTarFile(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// Push pushes the v1.Image img to a registry as dst.
func Push(img v1.Image, dst string, opt ...Option) error {
	o := makeOptions(opt...)
	tag, err := name.ParseReference(dst, o.Name...)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %w", dst, err)
	}
	return remote.Write(tag, img, o.Remote...)
}

// Upload pushes the v1.Layer to a given repo.
func Upload(layer v1.Layer, repo string, opt ...Option) error {
	o := makeOptions(opt...)
	ref, err := name.NewRepository(repo, o.Name...)
	if err != nil {
		return fmt.Errorf("parsing repo %q: %w", repo, err)
	}

	return remote.WriteLayer(ref, layer, o.Remote...)
}
