package mutate

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Drops docker specific properties
// See: https://github.com/opencontainers/image-spec/blob/main/config.md
func toOCIV1Config(config v1.Config) v1.Config {
	return v1.Config{
		User:         config.User,
		ExposedPorts: config.ExposedPorts,
		Env:          config.Env,
		Entrypoint:   config.Entrypoint,
		Cmd:          config.Cmd,
		Volumes:      config.Volumes,
		WorkingDir:   config.WorkingDir,
		Labels:       config.Labels,
		StopSignal:   config.StopSignal,
	}
}

func toOCIV1ConfigFile(cf *v1.ConfigFile) *v1.ConfigFile {
	return &v1.ConfigFile{
		Created:      cf.Created,
		Author:       cf.Author,
		Architecture: cf.Architecture,
		OS:           cf.OS,
		OSVersion:    cf.OSVersion,
		History:      cf.History,
		RootFS:       cf.RootFS,
		Config:       toOCIV1Config(cf.Config),
	}
}

// OCIImage mutates the provided v1.Image to be OCI compilant v1.Image
// Check image-spec to see which properties are ported and which are dropped.
// https://github.com/opencontainers/image-spec/blob/main/config.md
func OCIImage(base v1.Image) (v1.Image, error) {
	m, err := base.Manifest()
	if err != nil {
		return nil, err
	}

	manifest := m.DeepCopy()

	for i, layer := range manifest.Layers {
		switch layer.MediaType {
		case types.DockerLayer:
			manifest.Layers[i].MediaType = types.OCILayer
		case types.DockerUncompressedLayer:
			manifest.Layers[i].MediaType = types.OCIUncompressedLayer
		}
	}

	base = ImageManifest(base, manifest)
	base = MediaType(base, types.OCIManifestSchema1)
	base = ConfigMediaType(base, types.OCIConfigJSON)

	cfg, err := base.ConfigFile()
	if err != nil {
		return nil, err
	}
	cfg = toOCIV1ConfigFile(cfg)
	base, err = ConfigFile(base, cfg)
	if err != nil {
		return nil, err
	}
	return base, nil
}

// OCIImageIndex mutates the provided v1.ImageIndex to be OCI compilant v1.ImageIndex
func OCIImageIndex(base v1.ImageIndex) (v1.ImageIndex, error) {
	base = IndexMediaType(base, types.OCIImageIndex)
	mn, err := base.IndexManifest()
	if err != nil {
		return nil, err
	}

	removals := []v1.Hash{}
	addendums := []IndexAddendum{}

	for _, manifest := range mn.Manifests {
		if !manifest.MediaType.IsImage() {
			// it is not an image, leave it as is
			continue
		}
		img, err := base.Image(manifest.Digest)
		if err != nil {
			return nil, err
		}
		img, err = OCIImage(img)
		if err != nil {
			return nil, err
		}
		mt, err := img.MediaType()
		if err != nil {
			return nil, err
		}
		removals = append(removals, manifest.Digest)
		addendums = append(addendums, IndexAddendum{Add: img, Descriptor: v1.Descriptor{
			URLs:        manifest.URLs,
			MediaType:   mt,
			Annotations: manifest.Annotations,
			Platform:    manifest.Platform,
		}})
	}
	base = RemoveManifests(base, match.Digests(removals...))
	base = AppendManifests(base, addendums...)
	return base, nil
}
