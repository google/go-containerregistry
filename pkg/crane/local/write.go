package local

import (
	"bytes"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func Write(ref name.Reference, img v1.Image, options ...Option) (rerr error) {
	o, err := makeOptions(options...)
	if err != nil {
		return err
	}
	return o.path.AppendImage(img, layout.WithAnnotations(map[string]string{
		specsv1.AnnotationRefName: ref.String(),
	}))
}

func WriteLayer(layer v1.Layer, options ...Option) (rerr error) {
	o, err := makeOptions(options...)
	if err != nil {
		return err
	}
	digest, err := layer.Digest()
	if err != nil {
		return err
	}
	rc, err := layer.Compressed()
	if err != nil {
		return err
	}
	return o.path.WriteBlob(digest, rc)
}

func Put(ref name.Reference, t remote.Taggable, options ...Option) error {
	o, err := makeOptions(options...)
	if err != nil {
		return err
	}
	rmf, err := t.RawManifest()
	if err != nil {
		return err
	}
	digest, _, err := v1.SHA256(bytes.NewReader(rmf))
	if err != nil {
		return err
	}
	err = o.path.WriteBlob(digest, io.NopCloser(bytes.NewReader(rmf)))
	if err != nil {
		return err
	}
	mf, err := v1.ParseManifest(bytes.NewReader(rmf))
	if err != nil {
		return err
	}
	return o.path.AppendDescriptor(v1.Descriptor{
		Digest:    digest,
		MediaType: mf.MediaType,
		Size:      int64(len(rmf)),
		Annotations: map[string]string{
			specsv1.AnnotationRefName: ref.String(),
		},
	})
}
