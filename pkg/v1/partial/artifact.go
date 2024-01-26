package partial

import "github.com/google/go-containerregistry/pkg/v1/types"

type Artifact interface {
	Describable
	WithRawManifest
}

type WithMediaType interface {
	MediaType() (types.MediaType, error)
}
