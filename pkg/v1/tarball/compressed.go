package tarball

import (
	"compress/gzip"
	"io"

	"github.com/google/go-containerregistry/pkg/v1/v1util"
)

func isOpenerGzipped(opener Opener) (bool, error) {
	r, err := opener()
	if err != nil {
		return false, err
	}
	return v1util.IsGzipped(r)
}

func newGZOpener(opener Opener) Opener {
	return func() (io.ReadCloser, error) {
		return newGZReadCloser(opener)
	}
}

// gzReadCloser is a wrapper around gzip.Reader which closes the underlying
// reader of gzip.Reader on Close.
type gzReadCloser struct {
	gr *gzip.Reader
	// Underlying reader for gr.
	r io.ReadCloser
}

func newGZReadCloser(opener Opener) (*gzReadCloser, error) {
	r, err := opener()
	if err != nil {
		return nil, err
	}
	gr, err := gzip.NewReader(r)
	if err != nil {
		_ = r.Close()
		return nil, err
	}
	return &gzReadCloser{gr: gr, r: r}, nil
}

func (gzrc *gzReadCloser) Read(p []byte) (int, error) {
	return gzrc.gr.Read(p)
}

func (gzrc *gzReadCloser) Close() error {
	if err := gzrc.gr.Close(); err != nil {
		_ = gzrc.r.Close()
		return err
	}
	return gzrc.r.Close()
}
