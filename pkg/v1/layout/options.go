package layout

import "github.com/google/go-containerregistry/pkg/v1"

// LayoutOption is a functional option for Layout.
//
// TODO: We'll need to change this signature to support Sparse/Thin images.
// Or, alternatively, wrap it in a sparse.Image that returns an empty list for layers?
type LayoutOption func(*v1.Descriptor) error

func WithAnnotations(annotations map[string]string) LayoutOption {
	return func(desc *v1.Descriptor) error {
		if desc.Annotations == nil {
			desc.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			desc.Annotations[k] = v
		}

		return nil
	}
}

func WithURLs(urls []string) LayoutOption {
	return func(desc *v1.Descriptor) error {
		if desc.URLs == nil {
			desc.URLs = []string{}
		}
		for _, url := range urls {
			desc.URLs = append(desc.URLs, url)
		}

		return nil
	}
}

func WithPlatform(platform v1.Platform) LayoutOption {
	return func(desc *v1.Descriptor) error {
		desc.Platform = &platform
		return nil
	}
}
