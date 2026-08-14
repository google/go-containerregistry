// Copyright 2026 Google LLC All Rights Reserved.
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

package validate

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// isArtifact reports whether img is an artifact manifest rather than an
// image, e.g. a BuildKit SBOM/provenance attestation, which uses the OCI
// empty descriptor ({}) as its config.
func isArtifact(img v1.Image) (bool, error) {
	m, err := img.Manifest()
	if err != nil {
		return false, err
	}
	if m.ArtifactType != "" {
		return true, nil
	}
	return m.Config.MediaType != "" && !m.Config.MediaType.IsConfig(), nil
}

// validateArtifact checks the structural integrity of an artifact manifest:
// the manifest, config blob, and layer blobs must exist and match their
// declared digests and sizes. Image invariants (rootfs type, diff_ids,
// platform) do not apply to artifacts and are not checked.
func validateArtifact(img v1.Image, opt ...Option) error {
	errs := []string{}
	if err := validateManifest(img); err != nil {
		errs = append(errs, fmt.Sprintf("validating manifest: %v", err))
	}

	if err := validateArtifactConfig(img); err != nil {
		errs = append(errs, fmt.Sprintf("validating config: %v", err))
	}

	if err := validateArtifactLayers(img, opt...); err != nil {
		errs = append(errs, fmt.Sprintf("validating layers: %v", err))
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n\n"))
	}
	return nil
}

func validateArtifactConfig(img v1.Image) error {
	rc, err := img.RawConfigFile()
	if err != nil {
		return err
	}

	hash, size, err := v1.SHA256(bytes.NewReader(rc))
	if err != nil {
		return err
	}

	m, err := img.Manifest()
	if err != nil {
		return err
	}

	errs := []string{}
	if m.Config.Digest != hash {
		errs = append(errs, fmt.Sprintf("mismatched config digest: Manifest.Config.Digest=%s, SHA256(RawConfigFile())=%s", m.Config.Digest, hash))
	}

	if m.Config.Size != size {
		errs = append(errs, fmt.Sprintf("mismatched config size: Manifest.Config.Size=%d, len(RawConfigFile())=%d", m.Config.Size, size))
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func validateArtifactLayers(img v1.Image, opt ...Option) error {
	o := makeOptions(opt...)

	layers, err := img.Layers()
	if err != nil {
		return err
	}

	if o.fast {
		return layersExist(layers)
	}

	m, err := img.Manifest()
	if err != nil {
		return err
	}

	errs := []string{}
	for i, layer := range layers {
		rc, err := layer.Compressed()
		if err != nil {
			return err
		}
		digest, size, err := v1.SHA256(rc)
		rc.Close()
		if err != nil {
			return err
		}

		if m.Layers[i].Digest != digest {
			errs = append(errs, fmt.Sprintf("mismatched layer[%d] digest: Manifest.Layers[%d].Digest=%s, SHA256(Compressed())=%s", i, i, m.Layers[i].Digest, digest))
		}

		if m.Layers[i].Size != size {
			errs = append(errs, fmt.Sprintf("mismatched layer[%d] size: Manifest.Layers[%d].Size=%d, len(Compressed())=%d", i, i, m.Layers[i].Size, size))
		}
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}
