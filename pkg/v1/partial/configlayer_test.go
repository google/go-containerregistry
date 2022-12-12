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

package partial

import (
	"fmt"
	"io"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type testUIC struct {
	UncompressedImageCore
	configFile []byte
}

func (t testUIC) RawConfigFile() ([]byte, error) {
	return t.configFile, nil
}

type testCIC struct {
	CompressedImageCore
	configFile []byte
}

func (t testCIC) LayerByDigest(h v1.Hash) (CompressedLayer, error) {
	return nil, fmt.Errorf("no layer by diff ID %v", h)
}

func (t testCIC) RawConfigFile() ([]byte, error) {
	return t.configFile, nil
}

func TestConfigLayer(t *testing.T) {
	cases := []v1.Image{
		&compressedImageExtender{
			CompressedImageCore: testCIC{
				configFile: []byte("{}"),
			},
		},
		&uncompressedImageExtender{
			UncompressedImageCore: testUIC{
				configFile: []byte("{}"),
			},
		},
	}

	for _, image := range cases {
		hash, err := image.ConfigName()
		if err != nil {
			t.Fatalf("Error getting config name: %v", err)
		}

		if _, err := image.LayerByDigest(hash); err == nil {
			t.Error("LayerByDigest(config hash) returned nil error, wanted error")
		}

		layer, err := ConfigLayer(image)
		if err != nil {
			t.Fatalf("ConfigLayer: %v", err)
		}
		lr, err := layer.Uncompressed()
		if err != nil {
			t.Fatalf("Error getting uncompressed layer: %v", err)
		}
		zr, err := layer.Compressed()
		if err != nil {
			t.Fatalf("Error getting compressed layer: %v", err)
		}

		cfgLayerBytes, err := io.ReadAll(lr)
		if err != nil {
			t.Fatalf("Error reading config layer bytes: %v", err)
		}
		zcfgLayerBytes, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("Error reading config layer bytes: %v", err)
		}

		cfgFile, err := image.RawConfigFile()
		if err != nil {
			t.Fatalf("Error getting raw config file: %v", err)
		}

		if string(cfgFile) != string(cfgLayerBytes) {
			t.Errorf("Config file layer doesn't match raw config file")
		}
		if string(cfgFile) != string(zcfgLayerBytes) {
			t.Errorf("Config file layer doesn't match raw config file")
		}

		size, err := layer.Size()
		if err != nil {
			t.Fatalf("Error getting config layer size: %v", err)
		}
		if size != int64(len(cfgFile)) {
			t.Errorf("Size() = %d, want %d", size, len(cfgFile))
		}

		digest, err := layer.Digest()
		if err != nil {
			t.Fatalf("Digest() = %v", err)
		}
		if digest != hash {
			t.Errorf("ConfigLayer().Digest() != ConfigName(); %v, %v", digest, hash)
		}

		diffid, err := layer.DiffID()
		if err != nil {
			t.Fatalf("DiffId() = %v", err)
		}
		if diffid != hash {
			t.Errorf("ConfigLayer().DiffID() != ConfigName(); %v, %v", diffid, hash)
		}

		mt, err := layer.MediaType()
		if err != nil {
			t.Fatalf("Error getting config layer media type: %v", err)
		}

		if mt != types.OCIConfigJSON {
			t.Errorf("MediaType() = %v, want %v", mt, types.OCIConfigJSON)
		}
	}
}
