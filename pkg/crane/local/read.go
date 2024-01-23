package local

import (
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func refEqDescriptor(ref name.Reference, descriptor v1.Descriptor) bool {
	if _, ok := descriptor.Annotations[specsv1.AnnotationRefName]; ok {
		return true
	}
	return false
}

func Image(ref name.Reference, options ...Option) (v1.Image, error) {
	o, err := makeOptions(options...)
	if err != nil {
		return nil, err
	}
	desc, err := Head(ref, options...)
	if err != nil {
		return nil, err
	}
	return o.path.Image(desc.Digest)
}

// Pull returns a v1.Image of the remote image src.
func Pull(src string, options ...Option) (v1.Image, error) {
	ref, err := name.ParseReference(src)
	if err != nil {
		return nil, fmt.Errorf("parsing reference %q: %w", src, err)
	}
	return Image(ref, options...)
}

// Head returns a v1.Descriptor for the given reference
func Head(ref name.Reference, options ...Option) (*v1.Descriptor, error) {
	o, err := makeOptions(options...)
	if err != nil {
		return nil, err
	}

	idx, err := o.path.ImageIndex()
	if err != nil {
		return nil, err
	}
	im, err := idx.IndexManifest()
	if err != nil {
		return nil, err
	}
	for _, manifest := range im.Manifests {
		if refEqDescriptor(ref, manifest) {
			return &manifest, nil
		}
	}
	return nil, errors.New("could not find the image in oci-layout")
}
