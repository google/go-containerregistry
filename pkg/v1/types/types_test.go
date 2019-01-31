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
