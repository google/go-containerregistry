package types

import "testing"

func TestIsDistributable(t *testing.T) {
	for _, mt := range []MediaType{
		OCIRestrictedLayer,
		OCIUncompressedRestrictedLayer,
		DockerForeignLayer,
	} {
		if mt.IsDistributable() {
			t.Errorf("%s: should not be distributable", mt)
		}
	}

	for _, mt := range []MediaType{
		OCIContentDescriptor,
		OCIImageIndex,
		OCIManifestSchema1,
		OCIConfigJSON,
		OCILayer,
		OCIUncompressedLayer,
		DockerManifestSchema1,
		DockerManifestSchema1Signed,
		DockerManifestSchema2,
		DockerManifestList,
		DockerLayer,
		DockerConfigJSON,
		DockerPluginConfig,
		DockerUncompressedLayer,
	} {
		if !mt.IsDistributable() {
			t.Errorf("%s: should be distributable", mt)
		}
	}
}

func TestIsImage(t *testing.T) {
	for _, mt := range []MediaType{
		OCIManifestSchema1, DockerManifestSchema2,
	} {
		if !mt.IsImage() {
			t.Errorf("%s: should be image", mt)
		}
	}

	for _, mt := range []MediaType{
		OCIContentDescriptor,
		OCIImageIndex,
		OCIConfigJSON,
		OCILayer,
		OCIRestrictedLayer,
		OCIUncompressedLayer,
		OCIUncompressedRestrictedLayer,

		DockerManifestList,
		DockerLayer,
		DockerConfigJSON,
		DockerPluginConfig,
		DockerForeignLayer,
		DockerUncompressedLayer,
	} {
		if mt.IsImage() {
			t.Errorf("%s: should not be image", mt)
		}
	}
}

func TestIsIndex(t *testing.T) {
	for _, mt := range []MediaType{
		OCIImageIndex, DockerManifestList,
	} {
		if !mt.IsIndex() {
			t.Errorf("%s: should be index", mt)
		}
	}

	for _, mt := range []MediaType{
		OCIContentDescriptor,
		OCIConfigJSON,
		OCILayer,
		OCIRestrictedLayer,
		OCIUncompressedLayer,
		OCIUncompressedRestrictedLayer,
		OCIManifestSchema1,

		DockerManifestSchema2,
		DockerLayer,
		DockerConfigJSON,
		DockerPluginConfig,
		DockerForeignLayer,
		DockerUncompressedLayer,
	} {
		if mt.IsIndex() {
			t.Errorf("%s: should not be index", mt)
		}
	}
}
