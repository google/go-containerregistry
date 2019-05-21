package cache

import (
	"archive/tar"
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

func (fs *fscache) Put(l v1.Layer) error {
	d, err := l.Digest()
	if err != nil {
		return err
	}
	rc, err := l.Compressed()
	if err != nil {
		return err
	}
	defer rc.Close()
	f, err := os.Create(filepath.Join(fs.path, d.String()))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, tar.NewReader(rc))
	return err
}

func (fs *fscache) Get(h v1.Hash) (v1.Layer, error) {
	return tarball.LayerFromFile(filepath.Join(fs.path, h.String()))
}

func (fs *fscache) Delete(h v1.Hash) error {
	return os.Remove(filepath.Join(fs.path, h.String()))
}
