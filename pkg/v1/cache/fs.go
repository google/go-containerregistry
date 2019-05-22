package cache

import (
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type fscache struct {
	path string
}

// NewFilesystemCache returns a Cache implementation backed by files.
func NewFilesystemCache(path string) Cache {
	return &fscache{path}
}

func (fs *fscache) Put(l v1.Layer) (v1.Layer, error) {
	h, err := l.Digest()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(fs.path, h.String())
	return &layer{Layer: l, path: path}, nil
}

type layer struct {
	v1.Layer
	path string
}

func (l *layer) Compressed() (io.ReadCloser, error) {
	if err := os.MkdirAll(filepath.Dir(l.path), 0700); err != nil {
		return nil, err
	}
	f, err := os.Create(l.path)
	if err != nil {
		return nil, err
	}
	rc, err := l.Layer.Compressed()
	if err != nil {
		return nil, err
	}
	t := io.TeeReader(rc, f)
	closes := []func() error{rc.Close, f.Close}
	return &readcloser{t: t, closes: closes}, nil
}

type readcloser struct {
	t      io.Reader
	closes []func() error
}

func (rc *readcloser) Read(b []byte) (int, error) {
	return rc.t.Read(b)
}

func (rc *readcloser) Close() error {
	// Call all Close methods, even if any returned an error. Return the
	// first returned error.
	var err error
	for _, c := range rc.closes {
		lastErr := c()
		if err == nil {
			err = lastErr
		}
	}
	return err
}

func (fs *fscache) Get(h v1.Hash) (v1.Layer, error) {
	l, err := tarball.LayerFromFile(filepath.Join(fs.path, h.String()))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	return l, err
}

func (fs *fscache) Delete(h v1.Hash) error {
	return os.Remove(filepath.Join(fs.path, h.String()))
}
