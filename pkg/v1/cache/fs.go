package cache

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	v1 "github.com/google/go-containerregistry/pkg/v1"
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
	digest, err := l.Digest()
	if err != nil {
		return nil, err
	}
	diffID, err := l.DiffID()
	if err != nil {
		return nil, err
	}
	return &layer{
		Layer:  l,
		path:   fs.path,
		digest: digest,
		diffID: diffID,
	}, nil
}

type layer struct {
	v1.Layer
	path           string
	digest, diffID v1.Hash
}

func (l *layer) create(h v1.Hash) (io.WriteCloser, error) {
	if err := os.MkdirAll(l.path, 0700); err != nil {
		return nil, err
	}
	return os.Create(cachepath(l.path, h))
}

func (l *layer) Compressed() (io.ReadCloser, error) {
	f, err := l.create(l.digest)
	if err != nil {
		return nil, err
	}
	rc, err := l.Layer.Compressed()
	if err != nil {
		return nil, err
	}
	return &readcloser{
		t:      io.TeeReader(rc, f),
		closes: []func() error{rc.Close, f.Close},
	}, nil
}

func (l *layer) Uncompressed() (io.ReadCloser, error) {
	f, err := l.create(l.diffID)
	if err != nil {
		return nil, err
	}
	rc, err := l.Layer.Uncompressed()
	if err != nil {
		return nil, err
	}
	return &readcloser{
		t:      io.TeeReader(rc, f),
		closes: []func() error{rc.Close, f.Close},
	}, nil
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
	l, err := tarball.LayerFromFile(cachepath(fs.path, h))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	digest, _ := l.Digest()
	if err == io.ErrUnexpectedEOF || digest != h {
		// Delete and return ErrNotFound because the layer was incomplete.
		if err := fs.Delete(h); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	return l, err
}

func (fs *fscache) Delete(h v1.Hash) error {
	err := os.Remove(cachepath(fs.path, h))
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}

func cachepath(path string, h v1.Hash) string {
	var file string
	if runtime.GOOS == "windows" {
		file = fmt.Sprintf("%s-%s", h.Algorithm, h.Hex)
	} else {
		file = h.String()
	}
	return filepath.Join(path, file)
}
