package zstd

import (
	"bufio"
	"bytes"
	"github.com/google/go-containerregistry/internal/and"
	"github.com/klauspost/compress/zstd"
	"io"
)

var MagicHeader = []byte{'\x28', '\xb5', '\x2f', '\xfd'}

// ReadCloser reads uncompressed input data from the io.ReadCloser and
// returns an io.ReadCloser from which compressed data may be read.
// This uses gzip.BestSpeed for the compression level.
func ReadCloser(r io.ReadCloser) io.ReadCloser {
	return ReadCloserLevel(r, 1)
}

// ReadCloserLevel reads uncompressed input data from the io.ReadCloser and
// returns an io.ReadCloser from which compressed data may be read.
func ReadCloserLevel(r io.ReadCloser, level int) io.ReadCloser {
	pr, pw := io.Pipe()

	// For highly compressible layers, gzip.Writer will output a very small
	// number of bytes per Write(). This is normally fine, but when pushing
	// to a registry, we want to ensure that we're taking full advantage of
	// the available bandwidth instead of sending tons of tiny writes over
	// the wire.
	// 64K ought to be small enough for anybody.
	bw := bufio.NewWriterSize(pw, 2<<16)

	// Returns err so we can pw.CloseWithError(err)
	go func() error {
		// TODO(go1.14): Just defer {pw,gw,r}.Close like you'd expect.
		// Context: https://golang.org/issue/24283
		gw, err := zstd.NewWriter(bw, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
		if err != nil {
			return pw.CloseWithError(err)
		}

		if _, err := io.Copy(gw, r); err != nil {
			defer r.Close()
			defer gw.Close()
			return pw.CloseWithError(err)
		}

		// Close gzip writer to Flush it and write gzip trailers.
		if err := gw.Close(); err != nil {
			return pw.CloseWithError(err)
		}

		// Flush bufio writer to ensure we write out everything.
		if err := bw.Flush(); err != nil {
			return pw.CloseWithError(err)
		}

		// We don't really care if these fail.
		defer pw.Close()
		defer r.Close()

		return nil
	}()

	return pr
}

// UnzipReadCloser reads compressed input data from the io.ReadCloser and
// returns an io.ReadCloser from which uncompessed data may be read.
func UnzipReadCloser(r io.ReadCloser) (io.ReadCloser, error) {
	gr, err := zstd.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &and.ReadCloser{
		Reader: gr,
		CloseFunc: func() error {
			// If the unzip fails, then this seems to return the same
			// error as the read.  We don't want this to interfere with
			// us closing the main ReadCloser, since this could leave
			// an open file descriptor (fails on Windows).
			gr.Close()
			return r.Close()
		},
	}, nil
}

// Is detects whether the input stream is compressed.
func Is(r io.Reader) (bool, error) {
	magicHeader := make([]byte, 4)
	n, err := r.Read(magicHeader)
	if n == 0 && err == io.EOF {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return bytes.Equal(magicHeader, MagicHeader), nil
}
