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

package build

import (
	"archive/tar"
	"bytes"
	gb "go/build"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/v1util"
)

const appPath = "/ko-app"

type Options struct {
	// TODO(mattmoor): Architectures?
	GetBase         func(string) (v1.Image, error)
	GetCreationTime func() (*v1.Time, error)
}

type gobuild struct {
	opt   Options
	build func(string) (string, error)
}

// NewGo returns a build.Interface implementation that:
//  1. builds go binaries named by importpath,
//  2. containerizes the binary on a suitable base,
func NewGo(opt Options) (Interface, error) {
	return &gobuild{opt, build}, nil
}

// IsSupportedReference implements build.Interface
//
// Only valid importpaths that provide commands (i.e., are "package main") are
// supported.
func (*gobuild) IsSupportedReference(s string) bool {
	p, err := gb.Import(s, gb.Default.GOPATH, gb.ImportComment)
	if err != nil {
		return false
	}
	return p.IsCommand()
}

func build(ip string) (string, error) {
	tmpDir, err := ioutil.TempDir("", "ko")
	if err != nil {
		return "", err
	}
	file := filepath.Join(tmpDir, "out")

	cmd := exec.Command("go", "build", "-o", file, ip)

	// Last one wins
	// TODO(mattmoor): GOARCH=amd64
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux")

	var output bytes.Buffer
	cmd.Stderr = &output
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		log.Printf("Unexpected error running \"go build\": %v\n%v", err, output.String())
		return "", err
	}
	return file, nil
}

func tarBinary(binary string) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	file, err := os.Open(binary)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	header := &tar.Header{
		Name: appPath,
		Size: stat.Size(),
		// Use a fixed Mode, so that this isn't sensitive to the directory and umask
		// under which it was created. Additionally, windows can only set 0222,
		// 0444, or 0666, none of which be executable.
		Mode: 0555,
	}
	// write the header to the tarball archive
	if err := tw.WriteHeader(header); err != nil {
		return nil, err
	}
	// copy the file data to the tarball
	if _, err := io.Copy(tw, file); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Build implements build.Interface
func (gb *gobuild) Build(s string) (v1.Image, error) {
	// Get the CreationTime
	creationTime, err := gb.opt.GetCreationTime()
	if err != nil {
		return nil, err
	}

	// Do the build into a temporary file.
	file, err := gb.build(s)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(filepath.Dir(file))

	// Construct a tarball with the binary.
	layerBytes, err := tarBinary(file)
	if err != nil {
		return nil, err
	}

	// Create a layer from that tarball.
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return v1util.NopReadCloser(bytes.NewBuffer(layerBytes)), nil
	})
	if err != nil {
		return nil, err
	}

	// Determine the appropriate base image for this import path.
	base, err := gb.opt.GetBase(s)
	if err != nil {
		return nil, err
	}

	// Augment the base image with our application layer.
	withApp, err := mutate.AppendLayers(base, layer)
	if err != nil {
		return nil, err
	}

	// Start from a copy of the base image's config file, and set
	// the entrypoint to our app.
	cfg, err := withApp.ConfigFile()
	if err != nil {
		return nil, err
	}
	cfg = cfg.DeepCopy()
	cfg.Config.Entrypoint = []string{appPath}
	image, err := mutate.Config(withApp, cfg.Config)
	if err != nil {
		return nil, err
	}

	if creationTime != nil {
		return mutate.CreatedAt(image, *creationTime)
	}

	return image, nil
}
